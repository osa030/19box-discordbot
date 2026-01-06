package bot

import (
	"context"
	"time"

	"github.com/bwmarrin/discordgo"
	zlog "github.com/rs/zerolog/log"
)

const (
	cmdRequestName        = "req"
	cmdRequestDescription = "楽曲リクエスト受付コマンド"
	cmdOptionURLName      = "url"
	cmdOptionURLDesc      = "Spotifyの楽曲URLを入力してください"
)

// Registration and Unregistration

func (b *Bot) registerCommand() error {
	cmd := &discordgo.ApplicationCommand{
		Name:        cmdRequestName,
		Description: cmdRequestDescription,
		Options: []*discordgo.ApplicationCommandOption{
			{
				Type:        discordgo.ApplicationCommandOptionString,
				Name:        cmdOptionURLName,
				Description: cmdOptionURLDesc,
				Required:    true,
			},
		},
	}
	zlog.Info().Msgf("Registering command: %s", cmd.Name)
	_, err := b.session.ApplicationCommandCreate(b.session.State.User.ID, b.config.GuildID, cmd)
	if err != nil {
		return err
	}
	zlog.Info().Msgf("Command registered: %s", cmd.Name)
	return nil
}

func (b *Bot) unregisterCommands() error {
	commands, err := b.session.ApplicationCommands(b.session.State.User.ID, b.config.GuildID)
	if err != nil {
		return err
	}
	for _, cmd := range commands {
		err := b.session.ApplicationCommandDelete(b.session.State.User.ID, b.config.GuildID, cmd.ID)
		if err != nil {
			zlog.Error().Msgf("Command unregistration failed: %s", cmd.Name)
		} else {
			zlog.Info().Msgf("Command unregistered: %s", cmd.Name)
		}
	}
	return nil
}

// Handlers

func (b *Bot) handleCommand(s *discordgo.Session, i *discordgo.InteractionCreate) {
	if i.ApplicationCommandData().Name != cmdRequestName {
		return
	}

	// response to user that the bot is thinking (too slow to respond)
	err := s.InteractionRespond(i.Interaction, &discordgo.InteractionResponse{
		Type: discordgo.InteractionResponseDeferredChannelMessageWithSource,
		Data: &discordgo.InteractionResponseData{
			Flags: discordgo.MessageFlagsEphemeral,
		},
	})
	if err != nil {
		zlog.Error().Msgf("Defer response failed: %v", err)
		return
	}

	go b.requestTrack(i)
}

func (b *Bot) requestTrack(i *discordgo.InteractionCreate) {
	// get user information
	var userID string
	var displayName string
	if i.User != nil {
		userID = i.User.ID
		displayName = i.User.DisplayName()
	} else if i.Member != nil && i.Member.User != nil {
		userID = i.Member.User.ID
		displayName = i.Member.User.DisplayName()
	}
	if userID == "" {
		zlog.Error().Msg("User ID not found")
		b.responseUpdate(i, msgInternalError)
		return
	}
	zlog.Info().Msgf("Command from user: ID=%s, Name=%s", userID, displayName)

	// get request track URL
	options := i.ApplicationCommandData().Options
	if len(options) == 0 {
		zlog.Error().Msg("No options provided")
		b.responseUpdate(i, msgInternalError)
		return
	}
	trackURL := options[0].StringValue()
	zlog.Info().Msgf("Request trackURL=[%s] from user: ID=%s", trackURL, userID)

	// find token
	var token string
	token, ok := b.tokens.Load(userID)

	if !ok {
		zlog.Info().Msgf("Token not found for user: %s, generating new token", userID)
		// generate token
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		listenerId, err := b.client.Join(ctx, displayName, userID)
		if err != nil {
			zlog.Error().Msgf("Error 19box join: %v", err)
			b.responseUpdate(i, msgInternalError)
			return
		}

		token = listenerId
		b.tokens.Store(userID, token)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	success, responseMessage, responseCode, err := b.client.Request(ctx, token, trackURL)
	if err != nil {
		zlog.Error().Msgf("Error 19box request track: %v", err)
		b.responseUpdate(i, msgInternalError)
		return
	}

	zlog.Info().Msgf("Request track response: success=%v, message=%s, code=%s", success, responseMessage, responseCode)
	b.responseUpdate(i, responseMessage)
}
