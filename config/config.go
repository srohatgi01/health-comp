package config

import "os"

func Load() *Config {
	return &Config{
		PineconeKey: os.Getenv("PINECONE_API_KEY"),
	}
}
