package cmd

import (
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"mika/config"
	"mika/geo"
	"time"
)

// updategeoCmd represents the updategeo command
var updategeoCmd = &cobra.Command{
	Use:   "updategeo",
	Short: "Downloaded the latest geo database from MaxMind.com",
	Long:  `Downloaded the latest geo database from MaxMind.com`,
	Run: func(cmd *cobra.Command, args []string) {
		t0 := time.Now()
		key := viper.GetString(config.GeodbAPIKey)
		outPath := viper.GetString(config.GeodbPath)
		log.Infof("Starting download of MaxMind City database")
		if err := geo.DownloadDB(outPath, key); err != nil {
			log.Errorf("failed to download database: %s", err.Error())
		} else {
			d := time.Now().Sub(t0).String()
			log.Infof("Successfully downloaded geoip db to: %s (%s)", outPath, d)
		}
	},
}

func init() {
	rootCmd.AddCommand(updategeoCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// updategeoCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// updategeoCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
