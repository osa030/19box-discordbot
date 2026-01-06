package jukebox

import (
	"context"
	"net/http"
	"sync"

	"connectrpc.com/connect"
	"github.com/cockroachdb/errors"
	v1 "github.com/osa030/19box-discordbot/internal/gen/jukebox/v1"
	"github.com/osa030/19box-discordbot/internal/gen/jukebox/v1/jukeboxv1connect"
	zlog "github.com/rs/zerolog/log"
)

type NotificationType int

const (
	NotificationTypeSessionStart NotificationType = iota
	NotificationTypeSessionEnd
	NotificationTypeTrackStart
	NotificationTypeStreamClosed
	NotificationTypeStreamError
)

type Notification struct {
	Type    NotificationType
	Session *v1.SessionInfo
	Track   *v1.TrackInfo
	Error   error
}
type Client struct {
	client        jukeboxv1connect.ListenerServiceClient
	stream        *connect.ServerStreamForClient[v1.Notification]
	notifications chan *Notification
	closeOnce     sync.Once
}

func NewClient(url string) *Client {
	return &Client{
		client:        jukeboxv1connect.NewListenerServiceClient(http.DefaultClient, url),
		notifications: make(chan *Notification, 10),
	}
}

func (c *Client) Join(ctx context.Context, displayName string, externalUserId string) (string, error) {
	joinResponse, err := c.client.Join(ctx, connect.NewRequest(&v1.JoinRequest{
		DisplayName:    displayName,
		ExternalUserId: externalUserId,
	}))
	if err != nil {
		zlog.Error().Msgf("Error 19box join: %v", err)
		return "", errors.Wrap(err, "error 19box join")
	}
	listenerId := joinResponse.Msg.ListenerId
	zlog.Debug().Msgf("19box join success: %s(%s)[%s]", displayName, externalUserId, listenerId)
	return listenerId, nil
}

func (c *Client) Request(ctx context.Context, listenerId string, trackId string) (bool, string, string, error) {
	requestTrackResponse, err := c.client.RequestTrack(ctx, connect.NewRequest(&v1.RequestTrackRequest{
		ListenerId: listenerId,
		TrackId:    trackId,
	}))
	if err != nil {
		zlog.Error().Msgf("Error 19box request track: %v", err)
		return false, "", "", errors.Wrap(err, "error 19box request")
	}
	zlog.Debug().Msgf("19box request track result: %s(%s)[%s](%s)", listenerId, trackId, requestTrackResponse.Msg.Message, requestTrackResponse.Msg.Code)
	return requestTrackResponse.Msg.Success, requestTrackResponse.Msg.Message, requestTrackResponse.Msg.Code, nil
}

func (c *Client) Subscribe(ctx context.Context) error {
	stream, err := c.client.SubscribeNotifications(ctx, connect.NewRequest(&v1.SubscribeNotificationsRequest{}))
	if err != nil {
		zlog.Error().Msgf("Error 19box subscribe notifications: %v", err)
		return errors.Wrap(err, "error 19box subscribe notifications")
	}
	c.stream = stream
	go func() {
		zlog.Info().Msg("Receiving notifications...")
		defer zlog.Info().Msg("Stopped receiving notifications")

		for c.stream.Receive() {
			jukeboxNotification := c.stream.Msg()
			notificationType := jukeboxNotification.GetType()
			sessionState := jukeboxNotification.GetSessionInfo().GetState()
			trackState := jukeboxNotification.GetTrackInfo().GetState()
			zlog.Info().Msgf("Received seqNo:[%d] notification(%v) session state(%v), track state(%v)", jukeboxNotification.GetSequenceNo(), notificationType, sessionState, trackState)

			notification := &Notification{
				Session: jukeboxNotification.GetSessionInfo(),
				Track:   jukeboxNotification.GetTrackInfo(),
			}

			switch notificationType {
			case v1.NotificationType_NOTIFICATION_TYPE_INITIAL_STATE, v1.NotificationType_NOTIFICATION_TYPE_CHANGE_STATE:
				if sessionState == v1.SessionState_SESSION_STATE_RUNNING {
					notification.Type = NotificationTypeSessionStart
					c.notifications <- notification
					continue
				}

				if sessionState == v1.SessionState_SESSION_STATE_TERMINATED {
					notification.Type = NotificationTypeSessionEnd
					c.notifications <- notification
					continue
				}

			case v1.NotificationType_NOTIFICATION_TYPE_CHANGE_TRACK:
				if trackState == v1.TrackState_TRACK_STATE_STARTED {
					notification.Type = NotificationTypeTrackStart
					c.notifications <- notification
					continue
				}
			}
		}

		if err := c.stream.Err(); err != nil {
			zlog.Error().Msgf("Error receiving notification: %v", err)
			c.notifications <- &Notification{
				Type:  NotificationTypeStreamError,
				Error: errors.Wrap(err, "error receiving notification"),
			}
			return
		}

		c.notifications <- &Notification{
			Type:  NotificationTypeStreamClosed,
			Error: errors.New("stream closed"),
		}

	}()
	return nil
}

func (c *Client) ReceiveNotifications() <-chan *Notification {
	return c.notifications
}

func (c *Client) Unsubscribe() {
	c.closeOnce.Do(func() {
		if c.stream != nil {
			_ = c.stream.Close()
			c.stream = nil
		}
		if c.notifications != nil {
			close(c.notifications)
			c.notifications = nil
		}
	})
}
