package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
)

func main() {
	// Load dotenv file.
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Failed to load .env file")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		log.Fatal("BOT_TOKEN environment variable is not set.")
	}

	openaiToken := os.Getenv("OPENAI_API_KEY")
	if openaiToken == "" {
		log.Fatal("OPENAI_API_KEY environment variable is not set.")
	}

	openaiClient := openai.NewClient(openaiToken)

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		log.Fatal(err)
	}

	// TODO: Remove in production.
	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		err := handleMessage(bot, openaiClient, update.Message)
		if err != nil {
			log.Printf("Failed to handle message: %v", err)
		}
	}
}

func handleMessage(bot *tgbotapi.BotAPI, openaiClient *openai.Client, message *tgbotapi.Message) error {
	// Check if update is a voice message.
	if message.Voice == nil {
		log.Printf("Not a voice message")
		return nil
	}

	// TODO: Is username personal data? Should I log it?
	log.Printf("Received a new voice message from %s", message.From.UserName)

	err := sendTypingAction(bot, message.Chat.ID)
	if err != nil {
		return fmt.Errorf("sending typing action: %v", err)
	}

	// Get the voice file.
	tgFileURL, err := bot.GetFileDirectURL(message.Voice.FileID)
	if err != nil {
		return fmt.Errorf("Failed to get voice file URL: %v", err)
	}

	// Download the voice file.
	origFilePath, err := downloadFile(tgFileURL)
	if err != nil {
		return fmt.Errorf("Failed to download voice file: %v", err)
	}

	// Transcode the voice file to mp3.
	transcodedFilePath, err := transcodeOggToMp3(origFilePath)
	if err != nil {
		return fmt.Errorf("Failed to transcode voice file to mp3: %v", err)
	}

	text, err := transcribeFile(openaiClient, transcodedFilePath)
	if err != nil {
		return fmt.Errorf("Failed to transcribe the voice file: %v", err)
	}

	err = sendTranscriptionResult(bot, message, text)
	if err != nil {
		return fmt.Errorf("Failed to send the transcription result: %v", err)
	}

	return nil
}

// Downloads file by provided URL and saves it to temporary file.
// Returns the path to the file or error.
func downloadFile(url string) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()

	tmpfile, err := ioutil.TempFile("", "voice-*.ogg")
	if err != nil {
		return "", err
	}

	if _, err := io.Copy(tmpfile, res.Body); err != nil {
		return "", err
	}

	return tmpfile.Name(), nil
}

func transcribeFile(openaiClient *openai.Client, filePath string) (string, error) {
	audioResponse, err := openaiClient.CreateTranscription(context.TODO(), openai.AudioRequest{
		Model:    "whisper-1",
		FilePath: filePath,
	})
	if err != nil {
		return "", err
	}

	return audioResponse.Text, nil
}

func sendTranscriptionResult(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string) error {
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyToMessageID = message.MessageID

	_, err := bot.Send(msg)
	if err != nil {
		return fmt.Errorf("Failed to send the transcription result: %v", err)
	}

	return nil
}

func transcodeOggToMp3(filePath string) (string, error) {
	tmpfile, err := ioutil.TempFile("", "voice-*.mp3")
	if err != nil {
		return "", err
	}
	tmpfile.Close()

	cmd := exec.Command("ffmpeg", "-y", "-i", filePath, "-acodec", "libmp3lame", "-ab", "320k", tmpfile.Name())
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return "", err
	}

	return tmpfile.Name(), nil
}

func sendTypingAction(bot *tgbotapi.BotAPI, chatID int64) error {
	action := tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping)
	_, err := bot.Request(action)
	if err != nil {
		return err
	}

	return nil
}
