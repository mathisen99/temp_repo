package internal

const (
	// Connection Registration
	RPL_WELCOME           = "001"
	RPL_YOURHOST          = "002"
	RPL_CREATED           = "003"
	RPL_MYINFO            = "004"
	RPL_ISUPPORT          = "005"
	RPL_USERHOST          = "302"
	RPL_ENDOFWHO          = "315"
	RPL_WHOISUSER         = "311"
	RPL_WHOISSERVER       = "312"
	RPL_WHOISOPERATOR     = "313"
	RPL_ENDOFWHOIS        = "318"
	RPL_WHOISCHANNELS     = "319"
	
	// Messages and States
	RPL_AWAY              = "301"
	RPL_UNAWAY            = "305"
	RPL_NOWAWAY           = "306"
	RPL_CHANNELMODEIS     = "324"
	
	// Channel Information
	RPL_NOTOPIC           = "331"
	RPL_TOPIC             = "332"
	RPL_TOPICWHOTIME      = "333"
	RPL_NAMREPLY          = "353"
	RPL_ENDOFNAMES        = "366"
	ERR_CHANOPRIVSNEEDED  = "482"
	
	// Connection Status
	RPL_LUSERCLIENT       = "251"
	RPL_LUSEROP           = "252"
	RPL_LUSERUNKNOWN      = "253"
	RPL_LUSERCHANNELS     = "254"
	RPL_LUSERME           = "255"
	RPL_LOCALUSERS        = "265"
	RPL_GLOBALUSERS       = "266"
	
	// MOTD
	RPL_MOTDSTART         = "375"
	RPL_MOTD              = "372"
	RPL_ENDOFMOTD         = "376"
	
	// Authentication
	RPL_LOGGEDIN          = "900"
	RPL_SASLSUCCESS       = "903"
	RPL_UMODEIS           = "221"
	RPL_HOSTHIDDEN        = "396"

	// Errors
	ERR_NOSUCHNICK        = "401"
	ERR_NOSUCHCHANNEL     = "403"
	ERR_CANNOTSENDTOCHAN  = "404"
	ERR_TOOMANYCHANNELS   = "405"
	ERR_NICKNAMEINUSE     = "433"
	ERR_BANNEDFROMCHAN    = "474"
	ERR_CHANNELISFULL     = "471"
	ERR_INVITEONLYCHAN    = "473"
	ERR_BADCHANNELKEY     = "475"
	
	// IRC Commands
	CMD_PING              = "PING"
	CMD_PONG              = "PONG"
	CMD_PRIVMSG           = "PRIVMSG"
	CMD_NOTICE            = "NOTICE"
	CMD_JOIN              = "JOIN"
	CMD_PART              = "PART"
	CMD_KICK              = "KICK"
	CMD_QUIT              = "QUIT"
	CMD_NICK              = "NICK"
	CMD_INVITE            = "INVITE"
	CMD_TOPIC             = "TOPIC"
	CMD_MODE              = "MODE"
	CMD_ERROR             = "ERROR"
)

const (
	BOT_VERSION           = "1.0.0" 
	
	DEFAULT_CONFIG_PATH   = "./data/config.toml"
	DEFAULT_SETTINGS_PATH = "./data/settings.toml"
	DEFAULT_PLUGINS_PATH  = "./plugins"
	
	DEFAULT_RECONNECT_DELAY = 5
	DEFAULT_CONNECT_TIMEOUT = 30
)