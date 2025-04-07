# Discord Bot
A custom Discord bot with a bunch of random small features for various discord servers.

## Getting Started

You will need 
- [Go](https://go.dev/)
- A [postgres](https://postgresql.org) database

## Running the bot

- Copy `.env.template` to `.env` and place your credentials in it.
- Run the bot (from built binaries, or from source with `go run .`)

On first start with a fresh database, the bot will create the necessary structures and stop running immediately, because it has not yet been configured. After the first run, add tokens (such as discord tokens) to the database config table. The full list of config values are available in [](./internal/config/keys.go) and [](./internal/config/secrets.go). If you use another tool to manage the bot process (such as systemctl or docker), you can also specify environment variables there.

## Internationalization

All messages sent through the bot can be translated into different languages. Command names, descriptions, prompts, etc are all shown in the user's own langauge if available. All ephemeral messages are shown in the user's own language. All public messages (those sent to a channel visible to more than one person) are sent in the server's preferred language.

Translations are listed in YAML files in [](./internal/i18n). Any time there are lists of values they are chosen at random for variation. These variations should all contain the same information but they are not always direct translations from other languages.