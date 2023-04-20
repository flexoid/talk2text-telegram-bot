# Talk2Text Telegram Bot

Talk2Text is a self-hosted Telegram bot that automatically transcribes voice messages into text and provides a summary of the transcriptions.

## Features

- Transcribes voice messages into text (using Whisper API, supports multiple languages)
- Provides a summary of the transcriptions (using OpenAI GPT3.5 API)
- Language detection for summarization
- Simple PIN-based authentication

## Authentication

The bot uses a simple PIN-based authentication mechanism to control access to its features. When a user interacts with the bot for the first time, they are prompted to enter the configured PIN code.

Once the user provides the correct PIN code, they are granted access to the bot's features. The bot remembers authenticated users by storing their user IDs in memory. If the bot is restarted, all authenticated users will have to re-authenticate.

Please read [[Developer notes]](#developer-notes) for warnings about this authentication approach.

## Running the bot

### Prerequisites

Before running the Talk2Text bot, make sure you have the following:

- A Telegram bot token obtained from BotFather
- An OpenAI API key

If you want to run the bot locally without Docker, you'll also need:
- Go development environment installed
- FFmpeg installed

Clone the repository and switch to the repository folder:

```bash
git clone https://github.com/flexoid/talk2text-telegram-bot
cd talk2text-telegram-bot
```

### Configuration

Create a .env file in the root directory of the project and add the following environment variables:

```bash
BOT_TOKEN=your_telegram_bot_token
OPENAI_API_KEY=your_openai_api_key
PIN_CODE=your_pin_code
```

Replace your_telegram_bot_token, your_openai_api_key, and your_pin_code with your actual Telegram bot token, OpenAI API key, and desired PIN code, respectively.

### Running locally

Build the project:

```go
go build
```

To run the bot, execute the binary generated during the build process:

```bash
./talk2text-telegram-bot
```

The bot should now be running, and you can interact with it through Telegram. To authenticate, send your configured PIN code to the bot.

### Running on Docker

Make sure you have Docker installed and running on your machine.

Build the Docker image:

```bash
docker build -t talk2text-telegram-bot .
```

Run the Docker image:

```bash
docker run -d --name talk2text-telegram-bot talk2text-telegram-bot
```

The bot should now be running in a Docker container, and you can interact with it through Telegram. To authenticate, send your configured PIN code to the bot.

## Developer notes

This bot was initially developed for personal use in a single evening to experiment with the Whisper API. As a result, the repository currently lacks tests and may contain bugs or not be production-ready.

It's essential to note that the authentication approach employed in this bot is not highly secure, has potential vulnerabilities, and could lead to memory leaks. This implementation is intended for demonstration purposes only and should not be used in production environments without making improvements to the security and efficiency of the authentication process.
