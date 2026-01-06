package bot

import (
	"github.com/cockroachdb/errors"
	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

type DiscordBotConfig struct {
	Token   string `yaml:"token" validate:"required"`
	ForumID string `yaml:"forum_id" validate:"required"`
	GuildID string `yaml:"guild_id" validate:"required"`
}

// Validate validates the configuration.
func (c *DiscordBotConfig) Validate() error {
	if err := validate.Struct(c); err != nil {
		return errors.Wrap(err, "struct validation failed")
	}

	return nil
}
