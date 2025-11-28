package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
)

type Config struct {
	Db_url           string
	Current_username string
}

const configFileName = ".gatorconfig.json"

// Exported Functions
func Read() Config {

	filePath := getConfigFilePath()
	if filePath == nil {
		return Config{}
	}

	file, err := os.ReadFile(*filePath)
	if err != nil {
		fmt.Println("There was an issue reading the gatorconfig")
		fmt.Println(err)
		return Config{}
	}

	config := Config{}
	err = json.Unmarshal(file, &config)
	if err != nil {
		fmt.Println("There was an error unmarshalling the config")
		fmt.Println(err)
		return Config{}
	}

	return config
}

// Struct Methods
func (config Config) SetUser(username string) error {

	filePath := getConfigFilePath()
	if filePath == nil {
		return fmt.Errorf("error getting config file path")
	}

	config.Current_username = username
	write(config)

	return nil
}

// Private Functions
func getConfigFilePath() *string {

	homeDir, err := os.UserHomeDir()
	if err != nil {
		fmt.Println("There was an issue getting the home directory")
		fmt.Println(err)
		return nil
	}

	filePath := path.Join(homeDir, configFileName)
	return &filePath
}
func write(config Config) {

	filePath := getConfigFilePath()
	if filePath == nil {
		return
	}

	fileContents, err := json.MarshalIndent(config, "", "\t")
	if err != nil {
		fmt.Println("Something went wrong marshalling the config file")
		fmt.Println(err)
		return
	}

	err = os.WriteFile(*filePath, fileContents, 0444)
	if err != nil {
		fmt.Println("Something went wrong writing the config file")
		fmt.Println(err)
		return
	}
}
