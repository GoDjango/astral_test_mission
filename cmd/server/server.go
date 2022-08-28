package main

import (
	"astral_test_mission/internal/server"
	"github.com/jessevdk/go-flags"
	_ "github.com/lib/pq"
	log "github.com/sirupsen/logrus"
	"time"
)

func main() {

	var opts struct {
		Host string `long:"host" env:"HOST" default:"0.0.0.0"`
		Port string `long:"port" env:"PORT" default:"8080"`

		PostgresURL string `long:"postgres_url" env:"POSTGRES_URL" default:"postgresql://server:12345@0.0.0.0:5432/astral?sslmode=disable"`

		MinioURL string `long:"monio_url" env:"MINIO_URL" default:"http://minioadmin:minioadmin@0.0.0.0:9000/"`

		RootToken string `long:"root_token" env:"ROOT_TOKEN" default:"svdkbjnhkvdfsgksd456"`

		CacheUpdateTimeout int `long:"cache_update_timeout" env:"CACHE_UPDATE_TIMOUT" default:"60" help:"Cache update timeout, in seconds"`
	}

	if _, err := flags.Parse(&opts); err != nil {
		log.Fatalf("Failed to parse args: %s", err)
	}

	db := server.NewDB(opts.PostgresURL)
	defer db.Close()

	cache := server.NewCache(db, time.Duration(opts.CacheUpdateTimeout)*time.Second)
	fs, err := server.NewFileStorage(opts.MinioURL)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(server.Run(opts.Host, opts.Port, opts.RootToken, db, fs, cache))
}
