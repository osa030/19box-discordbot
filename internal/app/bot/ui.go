package bot

import (
	"fmt"
	"strings"
	"time"

	"github.com/bwmarrin/discordgo"
	v1 "github.com/osa030/19box-discordbot/internal/gen/jukebox/v1"
)

const (
	spotifyColor = 0x1DB954 // Spotifyの緑色

	// Message Templates
	msgSessionStartTitle = "🎵 session(%s)"
	msgSessionStartBody  = "🔊 セッションを開始しました。\n\n🔚: %s\n"
	msgSessionEndBody    = "🔊 セッションは終了しました。\n\n本日のプレイリストはコチラです。\n"
	msgNowPlayingBody    = "🎙️ nowplaying「%s」%s\n\n%s\n"
	msgRequesterUser     = "selected by <@%s>"
	msgRequesterName     = "selected by %s"
	msgInternalError     = "受付に失敗しました(内部エラー)"
	msgTimeUndetermined  = "終了時間未定"
	msgTimeScheduled     = "%s終了予定"
	msgActivityName      = "19box Discord Bot"
	msgActivityState     = "🎵 Spotifyの曲を共有中"

	// Embed constants
	embedPlaylistTitle = "🎶 %s"
	embedTrackTitle    = "🎵 %s"
	embedArtistPrefix  = "🎤 %s"
	embedKeywordField  = "Keyword"

	// Time formats
	timeFormatTopicTitle = "2006-01-02 15:04"
	timeFormatDisplay    = "15:04"
)

var (
	spotifyFooter = &discordgo.MessageEmbedFooter{
		Text:    "Spotify",
		IconURL: "https://storage.googleapis.com/pr-newsroom-wp/1/2023/05/Spotify_Primary_Logo_RGB_Green.png",
	}
)

// UI handling logic

func formatSessionEnd(endTime string) string {
	if endTime == "" {
		return msgTimeUndetermined
	}
	t, err := time.Parse(time.RFC3339, endTime)
	if err != nil {
		return msgTimeUndetermined
	}
	return fmt.Sprintf(msgTimeScheduled, t.Local().Format(timeFormatDisplay))
}

func createSessionMessage(content string, sessionInfo *v1.SessionInfo, thumbnailURL string) *discordgo.MessageSend {
	fields := createKeywordField(strings.Join(sessionInfo.Keywords, ", "))

	return &discordgo.MessageSend{
		Content: content,
		Embed: &discordgo.MessageEmbed{
			Title:  fmt.Sprintf(embedPlaylistTitle, sessionInfo.PlaylistName),
			URL:    sessionInfo.PlaylistUrl,
			Color:  spotifyColor,
			Fields: fields,
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: thumbnailURL,
			},
			Footer: spotifyFooter,
		},
	}
}

func createNowPlayingMessage(trackInfo *v1.TrackInfo, sessionInfo *v1.SessionInfo) *discordgo.MessageSend {
	artists := strings.Join(trackInfo.Artists, ", ")
	var requester string
	if trackInfo.RequesterExternalUserId != "" {
		requester = fmt.Sprintf(msgRequesterUser, trackInfo.RequesterExternalUserId)
	} else {
		requester = fmt.Sprintf(msgRequesterName, trackInfo.RequesterName)
	}

	content := fmt.Sprintf(msgNowPlayingBody, trackInfo.Name, artists, requester)
	fields := createKeywordField(strings.Join(sessionInfo.Keywords, ", "))

	return &discordgo.MessageSend{
		Content: content,
		Embed: &discordgo.MessageEmbed{
			Title:       fmt.Sprintf(embedTrackTitle, trackInfo.Name),
			Description: fmt.Sprintf(embedArtistPrefix, artists),
			URL:         trackInfo.Url,
			Color:       spotifyColor,
			Fields:      fields,
			Thumbnail: &discordgo.MessageEmbedThumbnail{
				URL: trackInfo.AlbumArtUrl,
			},
			Footer: spotifyFooter,
		},
	}
}

func createKeywordField(keywords string) []*discordgo.MessageEmbedField {
	if keywords == "" {
		return nil
	}
	return []*discordgo.MessageEmbedField{
		{
			Name:  embedKeywordField,
			Value: keywords,
		},
	}
}
