package bot

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/bwmarrin/discordgo"
	"github.com/cockroachdb/errors"
	v1 "github.com/osa030/19box-discordbot/internal/gen/jukebox/v1"
	jukebox "github.com/osa030/19box-discordbot/internal/jukebox"
	"github.com/puzpuzpuz/xsync/v3"
	zlog "github.com/rs/zerolog/log"
)

type Bot struct {
	config       *DiscordBotConfig
	session      *discordgo.Session
	guildIconURL string
	topicID      atomic.Pointer[string]
	client       *jukebox.Client
	errCh        chan error
	tokens       *xsync.MapOf[string, string]
	postedTracks *xsync.MapOf[string, bool]
	ctx          context.Context
	cancel       context.CancelFunc
	wg           sync.WaitGroup
}

func NewBot(
	cfg *DiscordBotConfig,
	client *jukebox.Client,
) (*Bot, error) {
	dg, err := discordgo.New("Bot " + cfg.Token)
	if err != nil {
		return nil, err
	}

	b := &Bot{
		config:       cfg,
		session:      dg,
		client:       client,
		errCh:        make(chan error, 1),
		tokens:       xsync.NewMapOf[string, string](),
		postedTracks: xsync.NewMapOf[string, bool](),
	}

	b.session.AddHandler(func(s *discordgo.Session, i *discordgo.InteractionCreate) {
		b.handleCommand(s, i)
	})
	b.session.Identify.Intents = discordgo.IntentsGuilds
	b.session.AddHandler(b.onReady)

	return b, nil
}

func (b *Bot) onReady(s *discordgo.Session, event *discordgo.Ready) {
	zlog.Info().Msgf("Logged in as: %v#%v", s.State.User.Username, s.State.User.Discriminator)
	guild, err := b.session.Guild(b.config.GuildID)
	if err != nil {
		zlog.Error().Msgf("Error getting guild: %v", err)
		b.handleError(err)
		return
	}
	b.guildIconURL = guild.IconURL("1024")
	zlog.Info().Msgf("Guild icon URL: %s", b.guildIconURL)

	if err := s.UpdateStatusComplex(discordgo.UpdateStatusData{
		Status: "online",
		Activities: []*discordgo.Activity{
			{
				Name:  msgActivityName,
				Type:  discordgo.ActivityTypeListening,
				State: msgActivityState,
			},
		},
	}); err != nil {
		zlog.Error().Msgf("Error updating status: %v", err)
	}
	if err := b.registerCommand(); err != nil {
		zlog.Error().Msgf("Error registering command: %v", err)
	}
	zlog.Info().Msgf("Logged in done!")
	go b.receiveNotifications()
	zlog.Info().Msgf("Notifications received started.")

}

func (b *Bot) Start() error {
	zlog.Info().Msg("Starting bot...")

	b.ctx, b.cancel = context.WithCancel(context.Background())

	err := b.client.Subscribe(b.ctx)
	if err != nil {
		zlog.Error().Msgf("Error subscribing to notifications: %v", err)
		return errors.Wrap(err, "error subscribing to notifications")
	}

	return b.session.Open()
}

func (b *Bot) receiveNotifications() {
	b.wg.Add(1)
	defer b.wg.Done()
	zlog.Info().Msg("Receiving notifications...")
	defer zlog.Info().Msg("Stopped receiving notifications")

	notifications := b.client.ReceiveNotifications()
	for {
		select {
		case <-b.ctx.Done():
			return
		case notification, ok := <-notifications:
			if !ok {
				return
			}
			zlog.Info().Msgf("Received notification: %v", notification.Type)
			switch notification.Type {
			case jukebox.NotificationTypeSessionStart:
				b.handleSessionStart(notification)
			case jukebox.NotificationTypeSessionEnd:
				b.handleSessionEnd(notification)
			case jukebox.NotificationTypeTrackStart:
				b.handleTrackStart(notification)
			case jukebox.NotificationTypeStreamClosed,
				jukebox.NotificationTypeStreamError:
				zlog.Error().Msgf("Error receiving notification: %v", notification.Error)
				b.handleError(notification.Error)
			default:
				zlog.Warn().Msgf("Unknown notification type: %v", notification.Type)
			}
		}
	}
}
func (b *Bot) handleSessionStart(notification *jukebox.Notification) {
	if b.getTopicID() != "" {
		return
	}

	// create topic name
	now := time.Now().Format(timeFormatTopicTitle)
	topicTitle := fmt.Sprintf(msgSessionStartTitle, now)
	zlog.Info().Msgf("Creating new topic[%s]", topicTitle)

	sessionInfo := notification.Session
	sessionEndTime := formatSessionEnd(sessionInfo.ScheduledEndTime)
	content := fmt.Sprintf(msgSessionStartBody, sessionEndTime)
	topicMessage := createSessionMessage(content, sessionInfo, b.guildIconURL)
	if err := b.createForumTopic(topicTitle, topicMessage); err != nil {
		zlog.Error().Msgf("Error creating forum topic: %v", err)
	}

	trackInfo := notification.Track
	if trackInfo != nil && (trackInfo.State == v1.TrackState_TRACK_STATE_STARTED || trackInfo.State == v1.TrackState_TRACK_STATE_PLAYING) {
		zlog.Info().Msgf("Track started: %s", trackInfo.Name)
		if err := b.postNowplaying(trackInfo, sessionInfo); err != nil {
			zlog.Error().Msgf("Error posting now playing: %v", err)
		}
	}
}

func (b *Bot) handleSessionEnd(notification *jukebox.Notification) {
	if b.getTopicID() == "" {
		return
	}

	sessionInfo := notification.Session
	topicMessage := createSessionMessage(msgSessionEndBody, sessionInfo, b.guildIconURL)
	if err := b.sendToTopic(topicMessage); err != nil {
		zlog.Error().Msgf("Error sending message to topic: %v", err)
	}
	b.setTopicID("")
	b.postedTracks.Clear()
}

func (b *Bot) handleTrackStart(notification *jukebox.Notification) {
	sessionInfo := notification.Session
	trackInfo := notification.Track
	if trackInfo != nil {
		zlog.Info().Msgf("Track started: %s", trackInfo.Name)
		if err := b.postNowplaying(trackInfo, sessionInfo); err != nil {
			zlog.Error().Msgf("Error posting now playing: %v", err)
		}
	}
}

func (b *Bot) postNowplaying(trackInfo *v1.TrackInfo, sessionInfo *v1.SessionInfo) error {

	trackID := trackInfo.TrackId
	if _, loaded := b.postedTracks.LoadOrStore(trackID, true); loaded {
		zlog.Warn().Msgf("Track already posted: %s", trackID)
		return nil
	}

	msg := createNowPlayingMessage(trackInfo, sessionInfo)
	if err := b.sendToTopic(msg); err != nil {
		zlog.Error().Msgf("Error sending now playing to topic: %v", err)
		return err
	}
	return nil
}

func (b *Bot) Stop() {
	zlog.Info().Msg("Stopping bot...")

	if b.cancel != nil {
		b.cancel()
	}

	err := b.unregisterCommands()
	if err != nil {
		zlog.Error().Msgf("Error removing global commands: %v", err)
	}

	zlog.Info().Msgf("Waiting for background processes...")
	b.wg.Wait()

	zlog.Info().Msgf("Closing session...")
	if err := b.session.Close(); err != nil {
		zlog.Error().Msgf("Error closing session: %v", err)
	}
	b.client.Unsubscribe()
	zlog.Info().Msg("Bot stopped")
}

func (b *Bot) handleError(err error) {
	b.errCh <- err
}

func (b *Bot) GetError() <-chan error {
	return b.errCh
}

func (b *Bot) responseUpdate(i *discordgo.InteractionCreate, content string) {
	_, err := b.session.InteractionResponseEdit(i.Interaction, &discordgo.WebhookEdit{
		Content: &content,
	})
	if err != nil {
		zlog.Error().Msgf("Error response update: %v", err)
	}
}

func (b *Bot) createForumTopic(title string, message *discordgo.MessageSend) error {
	s := b.session
	thread, err := s.ForumThreadStartComplex(b.config.ForumID, &discordgo.ThreadStart{
		Name:                title,
		AutoArchiveDuration: 1440, // 24時間
	}, message)

	if err != nil {
		zlog.Error().Msgf("Error creating forum topic: %v", err)
		return err
	}
	b.setTopicID(thread.ID)
	zlog.Info().Msgf("Created forum topic: %s (ID: %s)", thread.Name, thread.ID)
	return nil
}

func (b *Bot) sendToTopic(message *discordgo.MessageSend) error {
	topicID := b.getTopicID()
	if topicID == "" {
		return errors.New("topicID is not set")
	}

	s := b.session
	msg, err := s.ChannelMessageSendComplex(topicID, message)
	if err != nil {
		zlog.Error().Msgf("Error sending message to topic: %v", err)
		return err
	}
	zlog.Info().Msgf("Sent message to topic: %s (ID: %s)", msg.ID, msg.ChannelID)
	return nil
}

func (b *Bot) getTopicID() string {
	if p := b.topicID.Load(); p != nil {
		return *p
	}
	return ""
}

func (b *Bot) setTopicID(id string) {
	b.topicID.Store(&id)
}
