// Package main provides the discordbot entry point.
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/alecthomas/kingpin/v2"
	"github.com/joho/godotenv"
	"github.com/osa030/19box-discordbot/internal/app/bot"
	"github.com/osa030/19box-discordbot/internal/jukebox"
	"github.com/osa030/19box-discordbot/internal/logger"
	"github.com/osa030/19box-discordbot/internal/timezone"
	zlog "github.com/rs/zerolog/log"
)

const (
	defaultServerURL = "http://localhost:8080"
)

var (
	app     = kingpin.New("19box-discordbot", "19box jukebox discord client")
	server  = app.Flag("server", "Server address").Default(defaultServerURL).Envar("JUKEBOX_SERVER_URL").String()
	verbose = app.Flag("verbose", "Enable verbose (DEBUG) logging").Short('v').Envar("VERBOSE").Bool()
	logfile = app.Flag("logfile", "Path to log file (default: stdout)").Envar("LOGFILE").String()

	token   = app.Flag("token", "Discord bot token").Envar("DISCORD_BOT_TOKEN").String()
	guildID = app.Flag("guild-id", "Discord guild ID").Envar("DISCORD_GUILD_ID").String()
	forumID = app.Flag("forum-id", "Discord forum ID").Envar("DISCORD_FORUM_ID").String()
)

func init() {
	timezone.Init()

	// start command (default) - no need to store the command
	app.Command("start", "Start the bot (default)").Default()
}

func main() {
	// Load .env file if it exists (errors are ignored)
	_ = godotenv.Load()

	// Parse command
	kingpin.MustParse(app.Parse(os.Args[1:]))

	// Initialize logger
	if err := logger.Init(*verbose, *logfile); err != nil {
		panic(fmt.Sprintf("Failed to initialize logger: %v", err))
	}

	// Initialize config from flags/env vars
	cfg := bot.DiscordBotConfig{
		Token:   *token,
		GuildID: *guildID,
		ForumID: *forumID,
	}

	// Validate config
	if err := cfg.Validate(); err != nil {
		zlog.Error().Msgf("Config validation failed: %v", err)
		zlog.Info().Msg("Please provide required settings via flags or environment variables.")
		os.Exit(1)
	}

	zlog.Debug().Msgf("config.token:[%s]", cfg.Token)
	zlog.Debug().Msgf("config.forum_id:[%s]", cfg.ForumID)
	zlog.Debug().Msgf("config.guild_id:[%s]", cfg.GuildID)

	client := jukebox.NewClient(*server)

	bot, err := bot.NewBot(&cfg, client)
	if err != nil {
		zlog.Error().Msgf("Failed to init bot: %v", err)
		os.Exit(1)
	}
	if err := bot.Start(); err != nil {
		zlog.Error().Msgf("Failed to start bot: %v", err)
		os.Exit(1)
	}
	defer bot.Stop()

	// Wait for shutdown signal or session end
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case <-sigCh:
		zlog.Info().Msg("Received shutdown signal...")
	case err := <-bot.GetError():
		zlog.Error().Msgf("Bot error: %v", err)
	}
}
