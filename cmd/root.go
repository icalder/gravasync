// Copyright © 2017 NAME HERE <EMAIL ADDRESS>
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cmd

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/icalder/gravasync/gc"
	"github.com/icalder/gravasync/strava"

	homedir "github.com/mitchellh/go-homedir"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var cfgFile string
var username string
var password string

// rootCmd represents the base command when called without any subcommands
var rootCmd = &cobra.Command{
	Use:   "gravasync <username> <password>",
	Short: "Syncs activities from Garmin Connect to Strava, one at a time with prompts",
	Args: func(cmd *cobra.Command, args []string) error {
		if viper.GetString("garmin.username") == "" || viper.GetString("garmin.password") == "" {
			if username == "" || password == "" {
				if len(args) < 2 {
					return errors.New("GC username and password are required when not set in config")
				}
			}
		}
		if username == "" {
			username = viper.GetString("garmin.username")
		}
		if password == "" {
			password = viper.GetString("garmin.password")
		}
		return nil
	},
	Run: func(cmd *cobra.Command, args []string) {
		stravaClient := strava.NewStrava()
		// Do we have a Strava API access token?
		if viper.GetString("strava.accessToken") == "" {
			if err := stravaClient.Authorise(viper.GetString("strava.clientID"), viper.GetString("strava.clientSecret")); err != nil {
				panic(err)
			}
		} else {
			stravaClient.SetAccessToken(viper.GetString("strava.accessToken"))
		}

		garminClient := gc.NewGarminConnect(username, password)
		if err := garminClient.Login(); err != nil {
			panic(err)
		}
		if err := activityLoop(stravaClient, garminClient); err != nil {
			panic(err)
		}
	},
}

func activityLoop(stravaClient strava.Strava, garminClient gc.GarminConnect) error {
	for {
		activity := garminClient.NextActivity()
		fmt.Println(activity)
		switch choose() {
		case "y":
			tcxBytes, err := garminClient.ExportTCX(activity.ID)
			if err != nil {
				return err
			}
			if err := stravaClient.ImportTCX(activity.Name, false, tcxBytes); err != nil {
				return err
			}
		case "x":
			return nil
		}
	}
}

func choose() string {
	scanner := bufio.NewScanner(os.Stdin)
	fmt.Println("Upload (y), Skip (n) or Exit (x)?")
	for scanner.Scan() {
		input := strings.ToLower(scanner.Text())
		if input == "y" || input == "n" || input == "x" {
			return input
		}
		fmt.Println("Upload (y), Skip (n) or Exit (x)?")
	}
	return "x"
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.PersistentFlags().StringVar(&cfgFile, "config", "", "config file (default is $HOME/.gravasync.yaml)")
	rootCmd.PersistentFlags().StringVar(&username, "username", "", "username")
	rootCmd.PersistentFlags().StringVar(&password, "password", "", "password")
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := homedir.Dir()
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}

		// Search config in home directory with name ".gravasync" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigName(".gravasync")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Println("Using config file:", viper.ConfigFileUsed())
	}
}
