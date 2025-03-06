package connection

import (
	"context"
	"net"
	"time"

	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/bot"
	"ircbot/internal/config"
	"ircbot/internal/handlers"
	"ircbot/internal/logger"
)

// EstablishConnection attempts to connect to the IRC server with the provided configuration
func EstablishConnection(ctx context.Context, cfg *config.Config, initialOwnerNick string, isFirstRun bool) (*irc.Client, error) {
	connectionTimeout := time.Duration(internal.DEFAULT_CONNECT_TIMEOUT) * time.Second
	
	// Use DialContext so that dialing can be canceled with a timeout
	connectCtx, connectCancel := context.WithTimeout(ctx, connectionTimeout)
	defer connectCancel()
	
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(connectCtx, "tcp", cfg.Server)
	if err != nil {
		return nil, err
	}
	
	// Setup and run the IRC client.
	client := bot.SetupClient(conn, cfg)
	
	// If this is the first run, schedule a verification request to the owner
	if isFirstRun {
		go scheduleOwnerVerification(client, initialOwnerNick)
	}
	
	return client, nil
}

// scheduleOwnerVerification waits for the bot to connect and then sends a verification request
func scheduleOwnerVerification(client *irc.Client, initialOwnerNick string) {
	// Wait 10 seconds to ensure the bot has connected and joined channels
	time.Sleep(10 * time.Second)
	
	// Make sure we have a valid owner nickname
	if initialOwnerNick != "" {
		logger.Infof("Starting owner verification with nickname: %s", initialOwnerNick)
		handlers.SendOwnerVerificationRequest(client, initialOwnerNick)
	} else {
		logger.Errorf("Owner nickname is empty, unable to send verification request")
	}
}