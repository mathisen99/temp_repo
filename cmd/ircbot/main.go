package main

import (
	"context"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"ircbot/internal"
	"ircbot/internal/bot"
	"ircbot/internal/handlers"
	"ircbot/internal/initialization"
	"ircbot/internal/logger"
)

func main() {
	// Initialize the bot components
	cfg, initialOwnerNick, isFirstRun, err := initialization.Initialize()
	if err != nil {
		logger.Errorf("Initialization error: %v", err)
		os.Exit(1)
	}
	
	// Ensure all log files are closed on exit
	defer func() {
		logger.CloseLogFile()
		logger.CloseAllChatLogs()
		logger.Infof("All log files closed")
	}()

	// Create a cancellable context to manage shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Setup signal handling for graceful shutdown
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		<-sigs
		logger.Infof("Shutdown signal received, exiting...")
		cancel() // cancel the context
	}()

	// Main connection loop
	reconnectDelay := time.Duration(internal.DEFAULT_RECONNECT_DELAY) * time.Second
	connectionTimeout := time.Duration(internal.DEFAULT_CONNECT_TIMEOUT) * time.Second

	for {
		// Check for shutdown before attempting a new connection
		select {
		case <-ctx.Done():
			logger.Infof("Exiting connection loop due to shutdown signal.")
			return
		default:
		}

		logger.Infof("Attempting to connect to IRC server at %s...", cfg.Server)

		// Create a connection to the IRC server
		connectCtx, connectCancel := context.WithTimeout(ctx, connectionTimeout)
		dialer := net.Dialer{}
		conn, err := dialer.DialContext(connectCtx, "tcp", cfg.Server)
		connectCancel()

		if err != nil {
			logger.Errorf("Failed to connect: %v. Retrying in %s...", err, reconnectDelay)
			select {
			case <-ctx.Done():
				return // Exit if shutdown was requested during sleep
			case <-time.After(reconnectDelay):
				continue
			}
		}

		// Setup the IRC client
		client := bot.SetupClient(conn, cfg)

		// If this is the first run, schedule a verification request to the owner
		if isFirstRun && initialOwnerNick != "" {
			go func() {
				// Wait 10 seconds to ensure the bot has connected and joined channels
				time.Sleep(10 * time.Second)
				logger.Infof("Starting owner verification with nickname: %s", initialOwnerNick)
				handlers.SendOwnerVerificationRequest(client, initialOwnerNick)
			}()
		}

		// Run the client in a separate goroutine
		runErrCh := make(chan error, 1)
		go func() {
			runErrCh <- client.Run()
		}()

		// Wait until either the client stops or a shutdown is requested
		select {
		case <-ctx.Done():
			logger.Infof("Shutdown requested, closing connection.")
			if err := conn.Close(); err != nil {
				logger.Errorf("Error closing connection: %v", err)
			}

			// Wait for client.Run to terminate and log any errors
			if err := <-runErrCh; err != nil {
				logger.Errorf("client.Run terminated with error: %v", err)
			} else {
				logger.Infof("client.Run terminated cleanly")
			}
			return
		case err := <-runErrCh:
			if err != nil {
				logger.Errorf("IRC client disconnected: %v", err)
			}
		}

		// Close the connection after client.Run exits
		if err = conn.Close(); err != nil {
			logger.Errorf("Error closing connection: %v", err)
		}

		logger.Warnf("Reconnecting in %s...", reconnectDelay)
		select {
		case <-ctx.Done():
			return // Exit if shutdown was requested during sleep
		case <-time.After(reconnectDelay):
			// Continue the loop
		}
	}
}
