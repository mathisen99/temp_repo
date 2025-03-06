package setup

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"ircbot/internal/handlers"
	"ircbot/internal/logger"
	"ircbot/internal/userlevels"
)

// SecuritySetup prompts the user for owner nickname and passphrase during first run
func SecuritySetup() (string, string) {
	asciiArt := `
    ███╗   ███╗██████╗  ██████╗ ████████╗
    ████╗ ████║██╔══██╗██╔═══██╗╚══██╔══╝
    ██╔████╔██║██████╔╝██║   ██║   ██║   
    ██║╚██╔╝██║██╔══██╗██║   ██║   ██║   
    ██║ ╚═╝ ██║██████╔╝╚██████╔╝   ██║   
    ╚═╝     ╚═╝╚═════╝  ╚═════╝    ╚═╝   
    `
	
	// Colors for better visuals
	cyan := logger.GetColorFunc("cyan")
	green := logger.GetColorFunc("green")
	yellow := logger.GetColorFunc("yellow")
	red := logger.GetColorFunc("red")
	white := logger.GetColorFunc("white")
	blue := logger.GetColorFunc("blue")
	
	fmt.Println(cyan(asciiArt))
	fmt.Println(blue("╔═══════════════════════════════════════════════╗"))
	fmt.Println(blue("║         ") + yellow("SECURITY SETUP - FIRST RUN") + blue("         ║"))
	fmt.Println(blue("╚═══════════════════════════════════════════════╝"))
	fmt.Println("")
	fmt.Println(white("This appears to be the ") + green("first run") + white(" of the bot."))
	fmt.Println(white("Let's set up security to protect your bot."))
	fmt.Println("")
	
	// Prompt for owner nickname
	fmt.Print(blue("[1/2] ") + white("Enter the IRC nickname of the bot owner: "))
	reader := bufio.NewReader(os.Stdin)
	ownerNick, err := reader.ReadString('\n')
	if err != nil {
		logger.Errorf("Error reading owner nickname: %v", err)
		os.Exit(1)
	}
	ownerNick = strings.TrimSpace(ownerNick)
	
	// Prompt for passphrase
	fmt.Println("")
	fmt.Println(white("A secure passphrase is required to confirm ownership."))
	fmt.Println(white("The bot will ask for this passphrase via private message."))
	fmt.Print(blue("[2/2] ") + white("Enter a passphrase: "))
	
	passphrase, err := reader.ReadString('\n')
	if err != nil {
		logger.Errorf("Error reading passphrase: %v", err)
		os.Exit(1)
	}
	passphrase = strings.TrimSpace(passphrase)

	fmt.Println("")
	fmt.Println(blue("╔═══════════════════════════════════════════════╗"))
	fmt.Println(blue("║         ") + green("SECURITY SETUP COMPLETE") + blue("          ║"))
	fmt.Println(blue("╚═══════════════════════════════════════════════╝"))
	fmt.Println("")
	fmt.Printf(green("✓ ") + white("Owner nickname: ") + yellow("%s\n"), ownerNick)
	fmt.Println(green("✓ ") + white("Passphrase configured successfully"))
	fmt.Println("")
	fmt.Println(red("⚠ IMPORTANT:"))
	fmt.Println(white("1. Keep this passphrase secure!"))
	fmt.Println(white("2. After verification, your full hostmask will be used for authentication"))
	fmt.Println(white("3. You can add other users with ") + yellow("!setlevel <hostmask> <level>"))
	fmt.Println("")
	fmt.Println(blue("═════════════════════════════════════════════════"))
	fmt.Println("")

	return ownerNick, passphrase
}

// SetupOwnerVerification sets up the initial owner verification
func SetupOwnerVerification() (string, bool) {
	// Check if this is the first run of the bot
	isFirstRun := true
	var initialOwnerNick string
	
	ownerNick, passphrase := SecuritySetup()
	initialOwnerNick = ownerNick
	handlers.SetVerificationPassphrase(passphrase)
	
	// Set the owner by nickname only for the first run
	// After verification, we'll use hostmask-based permissions
	userlevels.SetUserLevel(ownerNick, userlevels.Owner)
	
	return initialOwnerNick, isFirstRun
}