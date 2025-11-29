# Blog-Aggregator 'Gator'
A blog aggregator called gator. Built in Go and runs in the terminal. Created with the boot.dev guided project

It will pull the rss feed from blogs you give it. Very barebones.

## Required to Run
The app uses Postgres and Go, you'll need to have them installed.

1. Create the file `.gatorconfig.json` in your home directory. Paste the following

```
{
	"Db_url": "",
	"Current_username": ""
}
```

2. You'll need to create a postgres database called `gator`. Set the Db_url field in the json file to the database url.

Example: `postgres://username:password@localhost:5432/gator?sslmode=disable`

You'll need to setup credentials with postgres, if you haven't already done so.

## Installation
Install using the command
```
go install github.com/Ethanol2/gator@latest
```
Swap `@latest` with a specific version if desired. For example `@0.0.2`

Run `gator register <your username>` to create a user.

Add your first feed using `gator addFeed <feed name> <feed url>`

Feeds are tied to the user that created them.

## Commands

- `agg <fetch delay>` Fetches the latest posts from your followed feeds. Fetch delay is the time inbetween http requests. Don't set this too low to avoid overwhelming their servers, or incurring whatever protection they employ. Does not display repeated posts.
- `login <username>` Logs in as the user specified
- `register <username>` Registers a new user. Duplicate usernames will be rejected
- `reset` Clears all users, and all feeds created by those users. Use with extreme caution.
- `users` Lists all users
- `addFeed <feed name> <feed url>` Adds a feed. The current user will automatically follow the feed.
- `feeds` Lists all feeds
- `follow <feed url>` Follows the feed
- `following` Lists all followed feeds
- `unfollow <feed url>` Unfollows the feed
- `browse <post count> <offset>` Returns posts ordered by most recent. Doesn't display the content, just details. `post count` dictates the number, `offset` dictates the start point. Example: `browse 10 5` will return posts 6 to 16
