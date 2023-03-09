package main

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	openai "github.com/sashabaranov/go-openai"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type Authenticator interface {
	CheckAuth(userID int64) bool
	Authenticate(userID int64, text string) bool
}

func main() {
	logger, debug := setupLogger()
	defer logger.Sync()

	// Load dotenv file.
	err := godotenv.Load()
	if err != nil {
		logger.Debug("Could not load .env file, ignoring.")
	}

	botToken := os.Getenv("BOT_TOKEN")
	if botToken == "" {
		logger.Fatal("BOT_TOKEN environment variable is not set.")
	}

	openaiToken := os.Getenv("OPENAI_API_KEY")
	if openaiToken == "" {
		logger.Fatal("OPENAI_API_KEY environment variable is not set.")
	}

	pin := os.Getenv("PIN_CODE")
	if pin == "" {
		logger.Fatal("PIN_CODE environment variable is not set.")
	}

	openaiClient := openai.NewClient(openaiToken)

	bot, err := tgbotapi.NewBotAPI(botToken)
	if err != nil {
		logger.Fatal(err)
	}

	if debug {
		bot.Debug = true
	}

	logger.Infof("Authorized on account %s", bot.Self.UserName)

	auth := NewPinAuthenticator(pin)

	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 30

	updates := bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		err := handleMessage(bot, openaiClient, auth, logger, update.Message)
		if err != nil {
			logger.Errorf("Failed to handle message: %v", err)
		}
	}
}

func handleMessage(
	bot *tgbotapi.BotAPI,
	openaiClient *openai.Client,
	auth Authenticator,
	logger *zap.SugaredLogger,
	message *tgbotapi.Message,
) error {
	if !handleAuth(bot, auth, logger, message) {
		return nil
	}

	// Check if update is a voice message.
	if message.Voice == nil {
		logger.Debug("Not a voice message")
		return nil
	}

	// TODO: Check if user ID is not personally identifiable information.
	logger.Debugf("Received a new voice message from %s", message.From.ID)

	err := sendTypingAction(bot, message.Chat.ID)
	if err != nil {
		return fmt.Errorf("sending typing action: %v", err)
	}

	// Get the voice file.
	tgFileURL, err := bot.GetFileDirectURL(message.Voice.FileID)
	if err != nil {
		return fmt.Errorf("voice file URL: %v", err)
	}

	// Download the voice file.
	origFilePath, err := downloadFile(tgFileURL)
	if err != nil {
		return fmt.Errorf("downloading voice file: %v", err)
	}

	transcodedFilePath, err := transcodeOggToMp3(origFilePath)
	if err != nil {
		return fmt.Errorf("transcoding voice file to mp3: %v", err)
	}

	text, err := transcribeFile(openaiClient, transcodedFilePath)
	if err != nil {
		return fmt.Errorf("transcribing the voice file: %v", err)
	}

	transcriptionMessage, err := sendTranscriptionResult(bot, message, text)
	if err != nil {
		return fmt.Errorf("sending the transcription result: %v", err)
	}

	err = sendTypingAction(bot, message.Chat.ID)
	if err != nil {
		return fmt.Errorf("sending typing action before summarization: %v", err)
	}

	summary, err := summarizeText(openaiClient, text)
	if err != nil {
		return fmt.Errorf("summarizing the transcription: %v", err)
	}

	err = sendSummary(bot, transcriptionMessage, summary)
	if err != nil {
		return fmt.Errorf("sending the summary: %v", err)
	}

	return nil
}

func handleAuth(
	bot *tgbotapi.BotAPI,
	auth Authenticator,
	logger *zap.SugaredLogger,
	message *tgbotapi.Message,
) bool {
	if auth.CheckAuth(message.From.ID) {
		return true
	}

	if auth.Authenticate(message.From.ID, message.Text) {
		logger.Debugf("User %s authenticated successfully", message.From.ID)
		_, err := bot.Send(tgbotapi.NewMessage(message.Chat.ID, "You are now authenticated."))
		if err != nil {
			logger.Warnf("Failed to send authentication success message: %v", err)
		}

		return true
	}

	_, err := bot.Send(tgbotapi.NewMessage(message.Chat.ID, "You are not authenticated, please send valid PIN code."))
	if err != nil {
		logger.Warnf("Failed to send authentication failure message: %v", err)
	}

	return false
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

func sendTranscriptionResult(bot *tgbotapi.BotAPI, message *tgbotapi.Message, text string) (*tgbotapi.Message, error) {
	msg := tgbotapi.NewMessage(message.Chat.ID, text)
	msg.ReplyToMessageID = message.MessageID

	transcriptionMessage, err := bot.Send(msg)
	if err != nil {
		return nil, err
	}

	return &transcriptionMessage, nil
}

func sendSummary(bot *tgbotapi.BotAPI, transcriptionMessage *tgbotapi.Message, summary string) error {
	msg := tgbotapi.NewMessage(transcriptionMessage.Chat.ID, summary)
	msg.ReplyToMessageID = transcriptionMessage.MessageID

	_, err := bot.Send(msg)
	if err != nil {
		return err
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

func summarizeText(openaiClient *openai.Client, text string) (string, error) {
	summaryResponse, err := openaiClient.CreateChatCompletion(context.TODO(), openai.ChatCompletionRequest{
		Model: openai.GPT3Dot5Turbo0301,
		Messages: []openai.ChatCompletionMessage{
			{
				Role: "user",
				Content: "Detect the language of the provided message and generate the summary using" +
					" the same language. I don't need to know the detected language, only print the summary." +
					" Respond using detected language. The message is:\n\n" + text,
			},
		},
	})
	if err != nil {
		return "", err
	}

	if len(summaryResponse.Choices) == 0 {
		return "", fmt.Errorf("no choices returned")
	}

	return summaryResponse.Choices[0].Message.Content, nil
}

// Returns logger and whether it's in debug mode.
func setupLogger() (*zap.SugaredLogger, bool) {
	logCfg := zap.NewProductionConfig()
	logCfg.Sampling = nil

	level, err := zapcore.ParseLevel(os.Getenv("LOG_LEVEL"))
	if err == nil {
		logCfg.Level.SetLevel(level)
	}

	zapLogger := zap.Must(logCfg.Build())
	zap.ReplaceGlobals(zapLogger)

	sugar := zapLogger.Sugar()
	tgbotapi.SetLogger(TgBotLogWrapper{logger: sugar})

	return sugar, level <= zap.DebugLevel
}

type TgBotLogWrapper struct {
	logger *zap.SugaredLogger
}

func (l TgBotLogWrapper) Printf(format string, v ...interface{}) {
	l.logger.Debugf(format, v...)
}

func (l TgBotLogWrapper) Println(v ...interface{}) {
	l.logger.Debug(v...)
}
