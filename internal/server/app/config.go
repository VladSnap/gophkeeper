package app

import "flag"

type AppConfig struct {
	ServerAddress string
	DatabaseURI   string
}

func ParseFlags() (*AppConfig, error) {
	opts := new(AppConfig)

	flag.StringVar(&opts.ServerAddress, "a", ":8080", "listen address")
	flag.StringVar(&opts.DatabaseURI, "d", "", "database connection string")

	flag.Parse()

	return opts, nil
}
