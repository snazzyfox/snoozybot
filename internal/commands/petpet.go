package commands

import (
	"net/http"
	"net/url"
	"strings"

	dg "github.com/bwmarrin/discordgo"
)

var petpet = BotCommand{
	ApplicationCommand: dg.ApplicationCommand{
		Name: "petpet",
		Options: []*dg.ApplicationCommandOption{
			{Name: "user", Type: dg.ApplicationCommandOptionUser, Required: true},
		},
	},
	CommandHandler: func(cd *CommandData) error {
		user := cd.Option("user").UserValue(cd.Session)
		pngUrl := strings.Replace(user.AvatarURL(""), ".webp", ".png", 1)
		petpetUrl := "https://memeado.vercel.app/api/petpet?image=" + url.QueryEscape(pngUrl)
		resp, err := http.Get(petpetUrl)
		if err != nil {
			// can't really tell the user about this; it's a backend issue...
			return err
		}
		return cd.Respond(Response{
			Public: true,
			InteractionResponseData: dg.InteractionResponseData{
				Files: []*dg.File{{
					Name:        user.Username + "_petpet.gif",
					ContentType: resp.Header.Get("Content-Type"),
					Reader:      resp.Body,
				}},
			},
		})
	},
}
