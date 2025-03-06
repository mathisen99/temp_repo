package handlers

import (
	"gopkg.in/irc.v4"
	"ircbot/internal"
	"ircbot/internal/config"
	"ircbot/internal/logger"
	"ircbot/internal/security"
	"ircbot/internal/userlevels"
	"strings"
	"time"
)

var PendingVerifications = make(map[string]bool)
var VerificationPassphrase string

func SetVerificationPassphrase(passphrase string) {
	VerificationPassphrase = passphrase
}

// SetPendingVerification updates the pending verification status for a user.
func SetPendingVerification(userNick string, pending bool) {
	PendingVerifications[userNick] = pending
}

func checkPendingVerification(userNick string) bool {
	isPending, ok := PendingVerifications[userNick]
	
	if !ok {
		for pendingNick, pending := range PendingVerifications {
			if strings.EqualFold(userNick, pendingNick) {
				return pending
			}
		}
	}
	
	return isPending && ok
}

func SendOwnerVerificationRequest(c *irc.Client, ownerNick string) {
	SetPendingVerification(ownerNick, true)

	messages := []string{
		"🔒 SECURITY VERIFICATION 🔒",
		"Hello! I am your new IRC bot. Security verification is required.",
		"Please confirm you are the owner by replying with the passphrase",
		"that was shown in the terminal during first run setup.",
		"",
		"After verification, your full hostmask will be used for authentication,",
		"providing much stronger security than nickname-based verification.",
	}

	for i, msg := range messages {
		if strings.TrimSpace(msg) == "" {
			logger.Debugf("Skipping empty message at index %d for owner %s", i, ownerNick)
			continue
		}
		err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, ownerNick, msg)
		if err != nil {
			logger.Errorf("Failed to send verification message to owner %s at index %d: %v", ownerNick, i, err)
			return
		} else {
			// Log the outgoing message
			logger.LogPrivateMessage(ownerNick, "TO", msg)
		}
		time.Sleep(300 * time.Millisecond)
	}

	logger.Successf("Sent verification request to owner: %s", ownerNick)
	logger.Infof("After verification, owner's full hostmask will be used for security.")
}

func handleOwnerVerification(c *irc.Client, ownerNick, message string, m *irc.Message) {
	passphrase := strings.TrimSpace(message)
	hostmask := m.Prefix.String()

	if passphrase == VerificationPassphrase {
		passphraseHash, err := security.GenerateHash(passphrase)
		if err != nil {
			logger.Errorf("Failed to hash passphrase for %s: %v", ownerNick, err)
			errMsg := "Verification failed due to an internal error. Please restart the bot."
			if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, ownerNick, errMsg); err != nil {
				logger.Errorf("Failed to send internal error message to %s: %v", ownerNick, err)
			} else {
				logger.LogPrivateMessage(ownerNick, "TO", errMsg)
			}
			return
		}

		hostmasks := []config.HostmaskEntry{
			{
				Hostmask: hostmask,
				Level:    int(userlevels.Owner),
			},
		}

		settings := &config.Settings{
			OwnerVerified:  true,
			OwnerPasshash:  passphraseHash,
			OwnerHostmask:  hostmask,
			HostmaskLevels: true,
			Hostmasks:      hostmasks,
		}

		if err := config.SaveSettings(settings); err != nil {
			logger.Errorf("Failed to save verification settings for %s: %v", ownerNick, err)
			errMsg := "Verification failed due to an error saving settings. Please restart the bot."
			if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, ownerNick, errMsg); err != nil {
				logger.Errorf("Failed to send settings error message to %s: %v", ownerNick, err)
			} else {
				logger.LogPrivateMessage(ownerNick, "TO", errMsg)
			}
			return
		}

		userlevels.SetUserLevelByHostmask(hostmask, userlevels.Owner)
		SetPendingVerification(ownerNick, false)

		successMessages := []string{
			"✅ Verification successful!",
			"",
			"You are now confirmed as the bot owner with the hostmask:",
			hostmask,
			"",
			"This hostmask will be used for all future authentication,",
			"",
			"You can use the following command to set permission levels for other users:",
			"!setlevel <hostmask> <level>",
			"",
			"Available levels: owner, admin, regular, badboy",
		}

		for i, msg := range successMessages {
			if strings.TrimSpace(msg) == "" {
				logger.Debugf("Skipping empty success message at index %d for owner %s", i, ownerNick)
				continue
			}
			if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, ownerNick, msg); err != nil {
				logger.Errorf("Failed to send success message to owner %s at index %d: %v", ownerNick, i, err)
			} else {
				// Log the outgoing message
				logger.LogPrivateMessage(ownerNick, "TO", msg)
			}
			time.Sleep(300 * time.Millisecond)
		}

		logger.Successf("Owner %s successfully verified with hostmask: %s", ownerNick, hostmask)

	} else {
		failMessages := []string{
			"❌ Verification failed!",
			"",
			"The passphrase you provided did not match the one configured during setup.",
			"Please try again with the correct passphrase.",
			"",
			"If you've forgotten the passphrase, you'll need to restart the bot",
			"and go through the security setup process again.",
		}

		for i, msg := range failMessages {
			if strings.TrimSpace(msg) == "" {
				logger.Debugf("Skipping empty failure message at index %d for owner %s", i, ownerNick)
				continue
			}
			if err := c.Writef("%s %s :%s", internal.CMD_PRIVMSG, ownerNick, msg); err != nil {
				logger.Errorf("Failed to send failure message to owner %s at index %d: %v", ownerNick, i, err)
			} else {
				// Log the outgoing message
				logger.LogPrivateMessage(ownerNick, "TO", msg)
			}
			time.Sleep(300 * time.Millisecond)
		}

		logger.Warnf("Owner verification attempt failed for %s", ownerNick)
	}
}