package mywalki

import (
	"fmt"
	"github.com/dchote/gumble/gumble"
	"github.com/dchote/gumble/gumbleopenal"
	"github.com/dchote/gumble/gumbleutil"
	"github.com/kennygrant/sanitize"
	"net"
	"os"
	"strings"
	"time"
)

var MyLedStrip *LedStrip

func (b *Mywalki) Init() {
	b.Config.Attach(gumbleutil.AutoBitrate)
	b.Config.Attach(b)

	b.initGPIO()
	MyLedStrip, _ = NewLedStrip()
	fmt.Printf("%v %s\n", MyLedStrip.buf, MyLedStrip.display)
	b.Connect()
}

func (b *Mywalki) CleanUp() {
	b.Client.Disconnect()
	if SeeedStudio {
		MyLedStrip.ledCtrl(OnlineLED, OffCol)
		MyLedStrip.ledCtrl(ParticipantsLED, OffCol)
		MyLedStrip.ledCtrl(TransmitLED, OffCol)
		MyLedStrip.closePort()
	} else {
		b.LEDOffAll()
	}
}

func (b *Mywalki) Connect() {
	var err error
	b.ConnectAttempts++

	_, err = gumble.DialWithDialer(new(net.Dialer), b.Address, b.Config, &b.TLSConfig)
	if err != nil {
		fmt.Printf("Connection to %s failed (%s), attempting again in 10 seconds...\n", b.Address, err)
		b.ReConnect()
	} else {
		b.OpenStream()
	}
}

func (b *Mywalki) ReConnect() {
	if b.Client != nil {
		b.Client.Disconnect()
	}

	if b.ConnectAttempts < 100 {
		go func() {
			time.Sleep(10 * time.Second)
			b.Connect()
		}()
		return
	} else {
		fmt.Fprintf(os.Stderr, "Unable to connect, giving up\n")
		os.Exit(1)
	}
}

func (b *Mywalki) OpenStream() {
	// Audio
	if os.Getenv("ALSOFT_LOGLEVEL") == "" {
		os.Setenv("ALSOFT_LOGLEVEL", "0")
	}

	if stream, err := gumbleopenal.New(b.Client); err != nil {
		fmt.Fprintf(os.Stderr, "Stream open error (%s)\n", err)
		os.Exit(1)
	} else {
		b.Stream = stream
	}
}

func (b *Mywalki) ResetStream() {
	b.Stream.Destroy()

	// Sleep a bit and re-open
	time.Sleep(50 * time.Millisecond)

	b.OpenStream()
}

func (b *Mywalki) TransmitStart() {
	if b.IsConnected == false {
		return
	}

	b.IsTransmitting = true

	// turn on our transmit LED
	if SeeedStudio {
		MyLedStrip.ledCtrl(TransmitLED, TransmitCol)
	} else {
		b.LEDOn(b.TransmitLED)
	}


	b.Stream.StartSource()
}

func (b *Mywalki) TransmitStop() {
	if b.IsConnected == false {
		return
	}

	b.Stream.StopSource()

	if SeeedStudio {
		MyLedStrip.ledCtrl(TransmitLED, OffCol)
	} else {
		b.LEDOff(b.TransmitLED)
	}

	b.IsTransmitting = false
}

func (b *Mywalki) OnConnect(e *gumble.ConnectEvent) {
	b.Client = e.Client

	b.ConnectAttempts = 0

	b.IsConnected = true
	// turn on our online LED
	if SeeedStudio {
		MyLedStrip.ledCtrl(OnlineLED, OnlineCol)
	} else {
		b.LEDOn(b.OnlineLED)
	}

	fmt.Printf("Connected to %s (%d)\n", b.Client.Conn.RemoteAddr(), b.ConnectAttempts)
	if e.WelcomeMessage != nil {
		fmt.Printf("Welcome message: %s\n", esc(*e.WelcomeMessage))
	}

	if b.ChannelName != "" {
		b.ChangeChannel(b.ChannelName)
	}
}

func (b *Mywalki) OnDisconnect(e *gumble.DisconnectEvent) {
	var reason string
	switch e.Type {
	case gumble.DisconnectError:
		reason = "connection error"
	}

	b.IsConnected = false

	// turn off our LEDs
	if SeeedStudio {
		MyLedStrip.ledCtrl(OnlineLED, OffCol)
		MyLedStrip.ledCtrl(ParticipantsLED, OffCol)
		MyLedStrip.ledCtrl(TransmitLED, OffCol)
	} else {
		b.LEDOff(b.OnlineLED)
		b.LEDOff(b.ParticipantsLED)
		b.LEDOff(b.TransmitLED)
	}

	if reason == "" {
		fmt.Printf("Connection to %s disconnected, attempting again in 10 seconds...\n", b.Address)
	} else {
		fmt.Printf("Connection to %s disconnected (%s), attempting again in 10 seconds...\n", b.Address, reason)
	}

	// attempt to connect again
	b.ReConnect()
}

func (b *Mywalki) ChangeChannel(ChannelName string) {
	channel := b.Client.Channels.Find(ChannelName)
	if channel != nil {
		b.Client.Self.Move(channel)
	} else {
		fmt.Printf("Unable to find channel: %s\n", ChannelName)
	}
}

func (b *Mywalki) ParticipantLEDUpdate() {
	time.Sleep(100 * time.Millisecond)

	// If we have more than just ourselves in the channel, turn on the participants LED, otherwise, turn it off

	var participantCount = len(b.Client.Self.Channel.Users)

	if participantCount > 1 {
		fmt.Printf("Channel '%s' has %d participants\n", b.Client.Self.Channel.Name, participantCount)
		if SeeedStudio {
			MyLedStrip.ledCtrl(ParticipantsLED, ParticipantsCol)
		} else {
			b.LEDOn(b.ParticipantsLED)
		}

	} else {
		fmt.Printf("Channel '%s' has no other participants\n", b.Client.Self.Channel.Name)
		if SeeedStudio {
			MyLedStrip.ledCtrl(ParticipantsLED, OffCol)
		} else {
			b.LEDOff(b.ParticipantsLED)
		}
	}
}

func (b *Mywalki) OnTextMessage(e *gumble.TextMessageEvent) {
	fmt.Printf("Message from %s: %s\n", e.Sender.Name, strings.TrimSpace(esc(e.Message)))
}

func (b *Mywalki) OnUserChange(e *gumble.UserChangeEvent) {
	var info string

	switch e.Type {
	case gumble.UserChangeConnected:
		info = "connected"
	case gumble.UserChangeDisconnected:
		info = "disconnected"
	case gumble.UserChangeKicked:
		info = "kicked"
	case gumble.UserChangeBanned:
		info = "banned"
	case gumble.UserChangeRegistered:
		info = "registered"
	case gumble.UserChangeUnregistered:
		info = "unregistered"
	case gumble.UserChangeName:
		info = "changed name"
	case gumble.UserChangeChannel:
		info = "changed channel"
	case gumble.UserChangeComment:
		info = "changed comment"
	case gumble.UserChangeAudio:
		info = "changed audio"
	case gumble.UserChangePrioritySpeaker:
		info = "is priority speaker"
	case gumble.UserChangeRecording:
		info = "changed recording status"
	case gumble.UserChangeStats:
		info = "changed stats"
	}

	fmt.Printf("Change event for %s: %s (%d)\n", e.User.Name, info, e.Type)

	go b.ParticipantLEDUpdate()
}

func (b *Mywalki) OnPermissionDenied(e *gumble.PermissionDeniedEvent) {
	var info string
	switch e.Type {
	case gumble.PermissionDeniedOther:
		info = e.String
	case gumble.PermissionDeniedPermission:
		info = "insufficient permissions"
	case gumble.PermissionDeniedSuperUser:
		info = "cannot modify SuperUser"
	case gumble.PermissionDeniedInvalidChannelName:
		info = "invalid channel name"
	case gumble.PermissionDeniedTextTooLong:
		info = "text too long"
	case gumble.PermissionDeniedTemporaryChannel:
		info = "temporary channel"
	case gumble.PermissionDeniedMissingCertificate:
		info = "missing certificate"
	case gumble.PermissionDeniedInvalidUserName:
		info = "invalid user name"
	case gumble.PermissionDeniedChannelFull:
		info = "channel full"
	case gumble.PermissionDeniedNestingLimit:
		info = "nesting limit"
	}

	fmt.Printf("Permission denied: %s\n", info)
}

func (b *Mywalki) OnChannelChange(e *gumble.ChannelChangeEvent) {
	go b.ParticipantLEDUpdate()
}

func (b *Mywalki) OnUserList(e *gumble.UserListEvent) {
}

func (b *Mywalki) OnACL(e *gumble.ACLEvent) {
}

func (b *Mywalki) OnBanList(e *gumble.BanListEvent) {
}

func (b *Mywalki) OnContextActionChange(e *gumble.ContextActionChangeEvent) {
}

func (b *Mywalki) OnServerConfig(e *gumble.ServerConfigEvent) {
}

func esc(str string) string {
	return sanitize.HTML(str)
}
