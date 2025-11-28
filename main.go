package main

import (
	"context"
	"database/sql"
	"encoding/xml"
	"fmt"
	"html"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/Ethanol2/blog-aggregator/internal/config"
	"github.com/Ethanol2/blog-aggregator/internal/database"
	"github.com/google/uuid"
	_ "github.com/lib/pq"
)

// #region Structs

type state struct {
	db  *database.Queries
	cfg *config.Config
}

type command struct {
	Name string
	Args []string
}
type commands struct {
	Map map[string]func(*state, command) error
}

type RSSFeed struct {
	Channel struct {
		Title       string    `xml:"title"`
		Link        string    `xml:"link"`
		Description string    `xml:"description"`
		Item        []RSSItem `xml:"item"`
	} `xml:"channel"`
}

type RSSItem struct {
	Title       string `xml:"title"`
	Link        string `xml:"link"`
	Description string `xml:"description"`
	PubDate     string `xml:"pubDate"`
	Skip        bool
}

// #region General Logic

func main() {

	cmds := commands{Map: make(map[string]func(*state, command) error)}
	cmds.register("login", handlerLogin)
	cmds.register("register", handlerRegister)
	cmds.register("reset", handlerReset)
	cmds.register("users", handlerGetUsers)
	cmds.register("agg", middlewareLoggedIn(handlerAgg))
	cmds.register("addFeed", middlewareLoggedIn(handlerAddFeed))
	cmds.register("feeds", handlerFeeds)
	cmds.register("follow", middlewareLoggedIn(handlerFollow))
	cmds.register("following", middlewareLoggedIn(handlerFollowing))
	cmds.register("unfollow", middlewareLoggedIn(handlerUnfollow))
	cmds.register("browse", middlewareLoggedIn(handlerBrowse))

	cfg := config.Read()
	state := state{cfg: &cfg}

	db, err := sql.Open("postgres", state.cfg.Db_url)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	dbQueries := database.New(db)
	state.db = dbQueries

	if len(os.Args) > 1 {
		cmd := command{
			Name: strings.ToLower(os.Args[1]),
			Args: os.Args[2:],
		}

		if cmd.Name == "help" {
			for cmd := range cmds.Map {
				fmt.Println(cmd)
			}
			os.Exit(0)
		}

		err := cmds.run(&state, cmd)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	} else {
		fmt.Println("No command provided")
		os.Exit(1)
	}
	os.Exit(0)
}

func fetchFeed(ctx context.Context, feedURL string) (*RSSFeed, error) {
	client := &http.Client{
		Timeout: time.Second * 10,
	}
	req, err := http.NewRequestWithContext(ctx, "", feedURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "gator")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	bytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	feed := RSSFeed{}
	err = xml.Unmarshal(bytes, &feed)
	if err != nil {
		return nil, err
	}

	return &feed, nil
}
func (f *RSSFeed) clean() {
	f.Channel.Title = html.UnescapeString(f.Channel.Title)
	f.Channel.Description = html.UnescapeString(f.Channel.Description)

	for i, item := range f.Channel.Item {
		item.Description = html.UnescapeString(item.Description)
		item.Title = html.UnescapeString(item.Title)

		f.Channel.Item[i] = item
	}
}
func (f *RSSFeed) print() {
	fmt.Printf("%s\n%s\n\n", f.Channel.Title, f.Channel.Description)

	printCount := 0

	for _, item := range f.Channel.Item {
		if !item.Skip {
			fmt.Printf("%s\n%s\n\n", item.Title, item.Description)
			printCount++
		}
	}

	if printCount == 0 {
		fmt.Println("Nothing new...")
	}
}

// #region Command Support
func (c *commands) run(s *state, cmd command) error {
	if fnc, exists := c.Map[cmd.Name]; exists {
		return fnc(s, cmd)
	}

	return fmt.Errorf("command \"%s\" doesn't exist", cmd.Name)
}
func (c *commands) register(name string, f func(*state, command) error) {
	c.Map[strings.ToLower(name)] = f
}

func middlewareLoggedIn(handler func(s *state, cmd command, user database.User) error) func(*state, command) error {
	return func(s *state, cmd command) error {
		user, err := s.db.GetUser(context.Background(), s.cfg.Current_username)
		if err != nil {
			return err
		}

		return handler(s, cmd, user)
	}
}

func scrapeNextFeed(s *state, user database.User) error {

	feed, err := s.db.GetNextFeedToFetch(context.Background(), user.ID)
	if err != nil {
		return err
	}

	fmt.Printf("Fetching %s...\n\n", feed.Name)

	rssfeed, err := fetchFeed(context.Background(), feed.Url)
	if err != nil {
		return err
	}

	rssfeed.clean()

	for i, item := range rssfeed.Channel.Item {

		pubTime, err := time.Parse("Mon, 02 Jan 2006 15:04:05 -0700", item.PubDate)
		if err != nil {
			rssfeed.Channel.Item[i].Skip = true
			fmt.Println(err)
			continue
		}
		_, err = s.db.CreatePost(context.Background(), database.CreatePostParams{
			ID:          uuid.New(),
			CreatedAt:   time.Now(),
			Title:       rssfeed.Channel.Title,
			Description: rssfeed.Channel.Description,
			Url:         item.Link,
			PublishedAt: pubTime,
			FeedID:      feed.ID,
		})

		if err != nil {
			rssfeed.Channel.Item[i].Skip = true
			if strings.Contains(err.Error(), "posts_url_key") {
				continue
			}
			fmt.Println(err)
		}
	}

	err = s.db.MarkFeedFetched(context.Background(), database.MarkFeedFetchedParams{
		ID: feed.ID,
		LastFetchedAt: sql.NullTime{
			Time:  time.Now(),
			Valid: true,
		},
	})
	if err != nil {
		return err
	}

	rssfeed.print()

	return nil
}

// #region Command Handlers
func handlerLogin(s *state, cmd command) error {

	if len(cmd.Args) == 0 {
		return fmt.Errorf("no username provided")
	}

	user, err := s.db.GetUser(context.Background(), cmd.Args[0])
	if err != nil {
		return err
	}

	err = s.cfg.SetUser(user.Name)
	if err != nil {
		return err
	}

	fmt.Println("User has been set")
	return nil
}
func handlerRegister(s *state, cmd command) error {

	if len(cmd.Args) == 0 {
		return fmt.Errorf("no name provided")
	}

	user, err := s.db.CreateUser(context.Background(), database.CreateUserParams{ID: uuid.New(), CreatedAt: time.Now(), UpdatedAt: time.Now(), Name: cmd.Args[0]})
	if err != nil {
		return err
	}

	err = s.cfg.SetUser(user.Name)
	if err != nil {
		return err
	}

	fmt.Println("User was created")
	fmt.Println(user)

	return nil
}
func handlerReset(s *state, cmd command) error {

	err := s.db.DeleteUsers(context.Background())
	if err != nil {
		return err
	}

	fmt.Println("Users reset")
	return nil
}
func handlerGetUsers(s *state, cmd command) error {

	users, err := s.db.GetAllUsers(context.Background())
	if err != nil {
		return err
	}

	for _, user := range users {
		if s.cfg.Current_username == user.Name {
			fmt.Printf("%s (current)\n", user.Name)
		} else {
			fmt.Println(user.Name)
		}
	}

	return nil
}
func handlerFeeds(s *state, cmd command) error {

	feeds, err := s.db.GetAllFeeds(context.Background())
	if err != nil {
		return err
	}

	users := []database.User{}

	feedNameWidth := 8
	userNameWidth := 8

	for _, feed := range feeds {

		if feedNameWidth < len(feed.Name) {
			feedNameWidth = len(feed.Name)
		}

		user, err := s.db.GetUserWithID(context.Background(), feed.UserID)
		if err != nil {
			return err
		}

		users = append(users, user)

		if userNameWidth < len(user.Name) {
			userNameWidth = len(user.Name)
		}
	}

	fmt.Printf("\n%-*s | %-*s | %-*s\n", feedNameWidth, "Feed Name", userNameWidth, "User", 5, "URL")
	fmt.Println(strings.Repeat("-", feedNameWidth+userNameWidth+11))

	for i, feed := range feeds {
		fmt.Printf("%-*s | %-*s | %s\n", feedNameWidth, feed.Name, userNameWidth, users[i].Name, feed.Url)
	}

	fmt.Println()

	return nil
}

// #region User Command Handlers
func handlerFollow(s *state, cmd command, user database.User) error {

	if len(cmd.Args) < 1 {
		return fmt.Errorf("expected feed url -> follow <url>")
	}

	feed, err := s.db.GetFeedWithURL(context.Background(), cmd.Args[0])
	if err != nil {
		return err
	}

	insert, err := s.db.CreateFeedFollow(context.Background(), database.CreateFeedFollowParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		UserID:    user.ID,
		FeedID:    feed.ID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("%s followed %s\n", insert.UserName, insert.FeedName)

	return nil
}
func handlerFollowing(s *state, cmd command, user database.User) error {

	following, err := s.db.GetFollowing(context.Background(), user.ID)
	if err != nil {
		return err
	}

	for _, feed := range following {
		fmt.Println(feed.FeedName)
	}

	return nil
}
func handlerUnfollow(s *state, cmd command, user database.User) error {

	if len(cmd.Args) < 1 {
		return fmt.Errorf("expected a feed url -> unfollow <url>")
	}

	feed, err := s.db.GetFeedWithURL(context.Background(), cmd.Args[0])
	if err != nil {
		return err
	}

	err = s.db.RemoveFollowing(context.Background(), database.RemoveFollowingParams{
		UserID: user.ID,
		FeedID: feed.ID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Successfully unfollowed %s\n", feed.Name)
	return nil
}
func handlerAgg(s *state, cmd command, user database.User) error {

	if len(cmd.Args) < 1 {
		return fmt.Errorf("expected the time in between requests -> agg <wait time>")
	}

	dur, err := time.ParseDuration(cmd.Args[0])
	if err != nil {
		return err
	}

	fmt.Println("Starting to fetch feeds. Stop using Ctrl-C")

	ticker := time.NewTicker(dur)
	for ; ; <-ticker.C {
		err = scrapeNextFeed(s, user)
		if err != nil {
			return err
		}
	}
}
func handlerAddFeed(s *state, cmd command, user database.User) error {

	if len(cmd.Args) < 2 {
		return fmt.Errorf("expected name and feed url -> addFeed <name> <url>")
	}

	feed, err := s.db.AddFeed(context.Background(), database.AddFeedParams{
		ID:        uuid.New(),
		CreatedAt: time.Now(),
		Name:      cmd.Args[0],
		Url:       cmd.Args[1],
		UserID:    user.ID,
	})
	if err != nil {
		return err
	}

	fmt.Printf("Added feed %s\n", feed.Name)
	return handlerFollow(
		s,
		command{
			Args: []string{feed.Url},
		},
		user)
}
func handlerBrowse(s *state, cmd command, user database.User) error {

	limit := 2
	offset := 0

	if len(cmd.Args) == 1 {
		var err error
		limit, err = strconv.Atoi(cmd.Args[0])
		if err != nil {
			fmt.Println("Expected input: browse <post limit (optional, default 2)> <offset (optional, default 0)>")
			return err
		}

		if len(cmd.Args) == 2 {
			offset, err = strconv.Atoi(cmd.Args[0])
			if err != nil {

				fmt.Println("Expected input: browse <post limit (optional, default 2)> <offset (optional, default 0)>")
				return err
			}
		}
	}

	posts, err := s.db.GetPosts(context.Background(), database.GetPostsParams{
		UserID: user.ID,
		Limit:  int32(limit),
		Offset: int32(offset),
	})

	if err != nil {
		return err
	}

	for _, post := range posts {
		fmt.Printf("%s\n%s\n\n", post.Title, post.Description)
	}

	return nil
}
