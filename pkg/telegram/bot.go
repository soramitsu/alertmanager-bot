package telegram

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/go-kit/kit/log"
	"github.com/go-kit/kit/log/level"
	"github.com/hako/durafmt"
	"github.com/metalmatze/alertmanager-bot/pkg/alertmanager"
	"github.com/oklog/run"
	"github.com/prometheus/alertmanager/notify"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/alertmanager/types"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/tucnak/telebot"
)

const (
	commandStart = "/start"
	commandStop  = "/stop"
	commandHelp  = "/help"
	commandChats = "/chats"

	commandStatus     	= "/status"
	commandAlerts     	= "/alerts"
	commandSilences   	= "/silences"
	commandMute 	  	= "/mute"
	commandMuteDel    	= "/mute_del"
	commandEnvironments	= "/environments"
	commandProjects 	= "/projects"
	commandSilenceAdd 	= "/silence_add"
	commandSilence    	= "/silence"
	commandSilenceDel 	= "/silence_del"

	responseStart = "Hey, %s! I will now keep you up to date!\n" + commandHelp
	responseStop  = "Alright, %s! I won't talk to you again.\n" + commandHelp
	responseHelp  = `
I'm a Prometheus AlertManager Bot for Telegram. I will notify you about alerts.
You can also ask me about my ` + commandStatus + `, ` + commandAlerts + ` & ` + commandSilences + `

Available commands:
` + commandStart + ` - Subscribe for alerts.
` + commandStop + ` - Unsubscribe for alerts.
` + commandStatus + ` - Print the current status.
` + commandAlerts + ` - List all alerts.
` + commandSilences + ` - List all silences.
` + commandChats + ` - List all users and group chats that subscribed.
` + commandMute + ` - Mute environments and/or projects.
` + commandMuteDel + ` - Delete mute.
` + commandEnvironments + ` - List all environments.
` + commandProjects + ` - List all projects.
`
	ProjectAndEnvironmentRegexp  = `/mute environment\[(\w+(\s*,\s*\w+)*)\],[ ]?project\[(\w+(\s*,\s*\w+)*)\]`
	ProjectRegexp = `/mute project\[(\w+(\s*,\s*\w+)*)\]`
	EnvironmentRegexp = `/mute environment\[(\w+(\s*,\s*\w+)*)\]`
	EnvironmentValuesRegexp = `environment\[(.*?)\]`
	ProjectValuesRegexp = `project\[(.*?)\]`
)

// BotChatStore is all the Bot needs to store and read
type BotChatStore interface {
	List() ([]telebot.Chat, error)
	AddChat(telebot.Chat, []string, []string) error
	GetChatInfo(telebot.Chat) (ChatInfo, error)
	RemoveChat(telebot.Chat) error
	MuteEnvironments(telebot.Chat, []string, []string) error
	MuteProjects(telebot.Chat, []string, []string) error
	UnmuteEnvironment(telebot.Chat, string, []string) error
	UnmuteProject(telebot.Chat, string, []string) error
}

// Bot runs the alertmanager telegram
type Bot struct {
	addr         string
	admins       []int // must be kept sorted
	environments	[]string
	projects		[]string
	alertmanager *url.URL
	templates    *template.Template
	chats        BotChatStore
	logger       log.Logger
	revision     string
	startTime    time.Time

	telegram *telebot.Bot

	commandsCounter *prometheus.CounterVec
	webhooksCounter prometheus.Counter
}

// BotOption passed to NewBot to change the default instance
type BotOption func(b *Bot)

// NewBot creates a Bot with the UserStore and telegram telegram
func NewBot(chats BotChatStore, token string, admin int, opts ...BotOption) (*Bot, error) {
	bot, err := telebot.NewBot(token)
	if err != nil {
		return nil, err
	}

	commandsCounter := prometheus.NewCounterVec(prometheus.CounterOpts{
		Namespace: "alertmanagerbot",
		Name:      "commands_total",
		Help:      "Number of commands received by command name",
	}, []string{"command"})
	if err := prometheus.Register(commandsCounter); err != nil {
		return nil, err
	}

	b := &Bot{
		logger:          log.NewNopLogger(),
		telegram:        bot,
		chats:           chats,
		addr:            "127.0.0.1:8080",
		admins:          []int{admin},
		alertmanager:    &url.URL{Host: "localhost:9093"},
		commandsCounter: commandsCounter,
		// TODO: initialize templates with default?
	}

	for _, opt := range opts {
		opt(b)
	}

	return b, nil
}

// WithLogger sets the logger for the Bot as an option
func WithLogger(l log.Logger) BotOption {
	return func(b *Bot) {
		b.logger = l
	}
}

// WithAddr sets the internal listening addr of the bot's web server receiving webhooks
func WithAddr(addr string) BotOption {
	return func(b *Bot) {
		b.addr = addr
	}
}

// WithAlertmanager sets the connection url for the Alertmanager
func WithAlertmanager(u *url.URL) BotOption {
	return func(b *Bot) {
		b.alertmanager = u
	}
}

// WithTemplates uses Alertmanager template to render messages for Telegram
func WithTemplates(t *template.Template) BotOption {
	return func(b *Bot) {
		b.templates = t
	}
}

// WithRevision is setting the Bot's revision for status commands
func WithRevision(r string) BotOption {
	return func(b *Bot) {
		b.revision = r
	}
}

// WithStartTime is setting the Bot's start time for status commands
func WithStartTime(st time.Time) BotOption {
	return func(b *Bot) {
		b.startTime = st
	}
}

// WithExtraAdmins allows the specified additional user IDs to issue admin
// commands to the bot.
func WithExtraAdmins(ids ...int) BotOption {
	return func(b *Bot) {
		b.admins = append(b.admins, ids...)
		sort.Ints(b.admins)
	}
}

// WithEnvironments allows to define environments that are monitored by Prometheus
func WithEnvironments(environmentsToUse string) BotOption {
	return func(b *Bot) {
		p := strings.Replace(environmentsToUse, " ", "", -1)
		environmentsToSave := strings.Split(p, ",")
		b.environments = append(b.environments, environmentsToSave...)
		b.environments = append(b.environments, "other")
	}
}

// WithProjects allows to define projects that are monitored by Prometheus
func WithProjects(projectsToUse string) BotOption {
	return func(b *Bot) {
		p := strings.Replace(projectsToUse, " ", "", -1)
		projectsToSave := strings.Split(p, ",")
		b.projects = append(b.projects, projectsToSave...)
		b.projects = append(b.projects, "other")
	}
}

// SendAdminMessage to the admin's ID with a message
func (b *Bot) SendAdminMessage(adminID int, message string) {
	b.telegram.SendMessage(telebot.User{ID: adminID}, message, nil)
}

// isAdminID returns whether id is one of the configured admin IDs.
func (b *Bot) isAdminID(id int) bool {
	i := sort.SearchInts(b.admins, id)
	return i < len(b.admins) && b.admins[i] == id
}

// Run the telegram and listen to messages send to the telegram
func (b *Bot) Run(ctx context.Context, webhooks <-chan notify.WebhookMessage) error {
	commandSuffix := fmt.Sprintf("@%s", b.telegram.Identity.Username)
	//TODO: update
	commands := map[string]func(message telebot.Message){
		commandStart:    b.handleStart,
		commandStop:     b.handleStop,
		commandHelp:     b.handleHelp,
		commandChats:    b.handleChats,
		commandStatus:   b.handleStatus,
		commandAlerts:   b.handleAlerts,
		commandSilences: b.handleSilences,
		commandMute: b.handleMute,
		commandMuteDel: b.handleMuteDel,
		commandEnvironments: b.handleEnvironments,
		commandProjects: b.handleProjects,
	}

	// init counters with 0
	for command := range commands {
		b.commandsCounter.WithLabelValues(command).Add(0)
	}

	process := func(message telebot.Message) error {
		if message.IsService() {
			return nil
		}

		if !b.isAdminID(message.Sender.ID) {
			b.commandsCounter.WithLabelValues("dropped").Inc()
			return fmt.Errorf("dropped message from forbidden sender")
		}

		if err := b.telegram.SendChatAction(message.Chat, telebot.Typing); err != nil {
			return err
		}

		// Remove the command suffix from the text, /help@BotName => /help
		text := strings.Replace(message.Text, commandSuffix, "", -1)
		// Only take the first part into account, /help foo => /help
		text = strings.Split(text, " ")[0]

		level.Debug(b.logger).Log("msg", "message received", "text", text)

		// Get the corresponding handler from the map by the commands text
		handler, ok := commands[text]

		if !ok {
			b.commandsCounter.WithLabelValues("incomprehensible").Inc()
			b.telegram.SendMessage(
				message.Chat,
				"Sorry, I don't understand...",
				nil,
			)
			return nil
		}

		b.commandsCounter.WithLabelValues(text).Inc()
		handler(message)

		return nil
	}

	messages := make(chan telebot.Message, 100)
	b.telegram.Listen(messages, time.Second)

	var gr run.Group
	{
		gr.Add(func() error {
			return b.sendWebhook(ctx, webhooks)
		}, func(err error) {
		})
	}
	{
		gr.Add(func() error {
			for {
				select {
				case <-ctx.Done():
					return nil
				case message := <-messages:
					if err := process(message); err != nil {
						level.Info(b.logger).Log(
							"msg", "failed to process message",
							"err", err,
							"sender_id", message.Sender.ID,
							"sender_username", message.Sender.Username,
						)
					}
				}
			}
		}, func(err error) {
		})
	}

	return gr.Run()
}

// sendWebhook sends messages received via webhook to all subscribed chats
func (b *Bot) sendWebhook(ctx context.Context, webhooks <-chan notify.WebhookMessage) error {
	for {
		select {
		case <-ctx.Done():
			return nil
		case w := <-webhooks:
			//for _, alert := range w.Alerts {
			//	alertEnvironmentName := alert.Labels["environment"]
			//	alertProjectName := alert.Labels["project"]
			//
			//	environmentChats, err := b.chats.GetUsersForEnvironment(alertEnvironmentName)
			//	if err != nil {
			//		level.Error(b.logger).Log("msg", "failed to get users for provided environment", "err", err)
			//	}
			//
			//	projectChats, err := b.chats.GetUsersForProject(alertProjectName)
			//	if err != nil {
			//		level.Error(b.logger).Log("msg", "failed to get users for provided project", "err", err)
			//	}
			//
			//	uniqueChats := getUniqueChats(append(environmentChats, projectChats...))
			//
			//	dataToSend := &template.Data{
			//		Receiver:          w.Receiver,
			//		Status:            w.Status,
			//		Alerts:            []template.Alert{alert},
			//		GroupLabels:       w.GroupLabels,
			//		CommonLabels:      w.CommonLabels,
			//		CommonAnnotations: w.CommonAnnotations,
			//		ExternalURL:       w.ExternalURL,
			//	}
			//
			//	out, err := b.templates.ExecuteHTMLString(`{{ template "telegram.default" . }}`, dataToSend)
			//	if err != nil {
			//		level.Warn(b.logger).Log("msg", "failed to template alerts", "err", err)
			//		continue
			//	}
			//
			//	for _, chat := range uniqueChats {
			//		err = b.telegram.SendMessage(chat, b.truncateMessage(out), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
			//		if err != nil {
			//			level.Warn(b.logger).Log("msg", "failed to send message to subscribed chat", "err", err)
			//		}
			//	}
			//
			//}

			chats, err := b.chats.List()
			if err != nil {
				level.Error(b.logger).Log("msg", "failed to get chat list from store", "err", err)
				continue
			}

			if len(chats) > 0 {
				data := &template.Data{
					Receiver:          w.Receiver,
					Status:            w.Status,
					Alerts:            w.Alerts,
					GroupLabels:       w.GroupLabels,
					CommonLabels:      w.CommonLabels,
					CommonAnnotations: w.CommonAnnotations,
					ExternalURL:       w.ExternalURL,
				}

				out, err := b.templates.ExecuteHTMLString(`{{ template "telegram.default" . }}`, data)
				if err != nil {
					level.Warn(b.logger).Log("msg", "failed to template alerts", "err", err)
					continue
				}

				for _, chat := range chats {
					err = b.telegram.SendMessage(chat, b.truncateMessage(out), &telebot.SendOptions{ParseMode: telebot.ModeHTML})
					if err != nil {
						level.Warn(b.logger).Log("msg", "failed to send message to subscribed chat", "err", err)
					}
				}
			}
		}
	}
}

func (b *Bot) handleStart(message telebot.Message) {
	//if err := b.chats.Add(message.Chat); err != nil {
	//	level.Warn(b.logger).Log("msg", "failed to add chat to chat store", "err", err)
	//	b.telegram.SendMessage(message.Chat, "I can't add this chat to the subscribers list.", nil)
	//	return
	//}

	b.telegram.SendMessage(message.Chat, fmt.Sprintf(responseStart, message.Sender.FirstName), nil)
	level.Info(b.logger).Log(
		"user subscribed",
		"username", message.Sender.Username,
		"user_id", message.Sender.ID,
	)
}

func (b *Bot) handleStop(message telebot.Message) {
	//if err := b.chats.Remove(message.Chat); err != nil {
	//	level.Warn(b.logger).Log("msg", "failed to remove chat from chat store", "err", err)
	//	b.telegram.SendMessage(message.Chat, "I can't remove this chat from the subscribers list.", nil)
	//	return
	//}

	b.telegram.SendMessage(message.Chat, fmt.Sprintf(responseStop, message.Sender.FirstName), nil)
	level.Info(b.logger).Log(
		"user unsubscribed",
		"username", message.Sender.Username,
		"user_id", message.Sender.ID,
	)
}

func (b *Bot) handleHelp(message telebot.Message) {
	b.telegram.SendMessage(message.Chat, responseHelp, nil)
}

func (b *Bot) handleChats(message telebot.Message) {
	chats, err := b.chats.List()
	if err != nil {
		level.Warn(b.logger).Log("msg", "failed to list chats from chat store", "err", err)
		b.telegram.SendMessage(message.Chat, "I can't list the subscribed chats.", nil)
		return
	}

	list := ""
	for _, chat := range chats {
		if chat.IsGroupChat() {
			list = list + fmt.Sprintf("@%s\n", chat.Title)
		} else {
			list = list + fmt.Sprintf("@%s\n", chat.Username)
		}
	}

	b.telegram.SendMessage(message.Chat, "Currently these chat have subscribed:\n"+list, nil)
}

func (b *Bot) handleStatus(message telebot.Message) {
	s, err := alertmanager.Status(b.logger, b.alertmanager.String())
	if err != nil {
		level.Warn(b.logger).Log("msg", "failed to get status", "err", err)
		b.telegram.SendMessage(message.Chat, fmt.Sprintf("failed to get status... %v", err), nil)
		return
	}

	uptime := durafmt.Parse(time.Since(s.Data.Uptime))
	uptimeBot := durafmt.Parse(time.Since(b.startTime))

	b.telegram.SendMessage(
		message.Chat,
		fmt.Sprintf(
			"*AlertManager*\nVersion: %s\nUptime: %s\n*AlertManager Bot*\nVersion: %s\nUptime: %s",
			s.Data.VersionInfo.Version,
			uptime,
			b.revision,
			uptimeBot,
		),
		&telebot.SendOptions{ParseMode: telebot.ModeMarkdown},
	)
}

func (b *Bot) handleAlerts(message telebot.Message) {
	alerts, err := alertmanager.ListAlerts(b.logger, b.alertmanager.String())
	if err != nil {
		b.telegram.SendMessage(message.Chat, fmt.Sprintf("failed to list alerts... %v", err), nil)
		return
	}

	if len(alerts) == 0 {
		b.telegram.SendMessage(message.Chat, "No alerts right now! 🎉", nil)
		return
	}

	out, err := b.tmplAlerts(alerts...)
	if err != nil {
		return
	}

	err = b.telegram.SendMessage(message.Chat, b.truncateMessage(out), &telebot.SendOptions{
		ParseMode: telebot.ModeHTML,
	})
	if err != nil {
		level.Warn(b.logger).Log("msg", "failed to send message", "err", err)
	}
}

func (b *Bot) handleSilences(message telebot.Message) {
	silences, err := alertmanager.ListSilences(b.logger, b.alertmanager.String())
	if err != nil {
		b.telegram.SendMessage(message.Chat, fmt.Sprintf("failed to list silences... %v", err), nil)
		return
	}

	if len(silences) == 0 {
		b.telegram.SendMessage(message.Chat, "No silences right now.", nil)
		return
	}

	var out string
	for _, silence := range silences {
		out = out + alertmanager.SilenceMessage(silence) + "\n"
	}

	b.telegram.SendMessage(message.Chat, out, &telebot.SendOptions{ParseMode: telebot.ModeMarkdown})
}

func (b *Bot) handleMute(message telebot.Message) {
	//envToAlarm, prToAlarm, err := parseMuteCommand(message.Text, b.environments, b.projects)
	//if err != nil {
	//	b.telegram.SendMessage(message.Chat, fmt.Sprintf("failed to parse mute command... %v", err), nil)
	//	return
	//}

	//if len(envToAlarm) > 0 {
	//	for _, env := range envToAlarm {
	//		err := b.chats.AddUserToEnvironment(message.Chat, env)
	//		if err != nil {
	//			level.Warn(b.logger).Log("msg", "failed to subscribe user to environment", "err", err)
	//			b.telegram.SendMessage(message.Chat, fmt.Sprintf("failed to subscribe user to environments... %v", err), nil)
	//		}
	//	}
	//}
	//
	//if len(prToAlarm) > 0 {
	//	for _, pr := range prToAlarm {
	//		err := b.chats.AddUserToProject(message.Chat, pr)
	//		if err != nil {
	//			level.Warn(b.logger).Log("msg", "failed to subscribe user to project", "err", err)
	//			b.telegram.SendMessage(message.Chat, fmt.Sprintf("failed to subscribe user to project... %v", err), nil)
	//		}
	//	}
	//}
	//
	//if err := b.chats.Remove(message.Chat); err != nil {
	//	level.Warn(b.logger).Log("msg", "failed to remove user from getting all notifications", "err", err)
	//	b.telegram.SendMessage(message.Chat, fmt.Sprintf("failed to remove user from getting all notifications... %v", err), nil)
	//}

	b.telegram.SendMessage(message.Chat, "You were successfully subscribed to environments and/or projects", nil)

}

func (b *Bot) handleMuteDel(message telebot.Message) {
	// TODO
}

func (b *Bot) handleEnvironments(message telebot.Message) {
	b.telegram.SendMessage(message.Chat, fmt.Sprintf("The following environments are available: %s", b.environments), nil)
}

func (b *Bot) handleProjects(message telebot.Message) {
	b.telegram.SendMessage(message.Chat, fmt.Sprintf("The following projects are available: %s", b.projects), nil)
}

func (b *Bot) tmplAlerts(alerts ...*types.Alert) (string, error) {
	data := b.templates.Data("default", nil, alerts...)

	out, err := b.templates.ExecuteHTMLString(`{{ template "telegram.default" . }}`, data)
	if err != nil {
		return "", err
	}

	return out, nil
}

// Truncate very big message
func (b *Bot) truncateMessage(str string) string {
	truncateMsg := str
	if len(str) > 4095 { // telegram API can only support 4096 bytes per message
		level.Warn(b.logger).Log("msg", "Message is bigger than 4095, truncate...")
		// find the end of last alert, we do not want break the html tags
		i := strings.LastIndex(str[0:4080], "\n\n") // 4080 + "\n<b>[SNIP]</b>" == 4095
		if i > 1 {
			truncateMsg = str[0:i] + "\n<b>[SNIP]</b>"
		} else {
			truncateMsg = "Message is too long... can't send.."
			level.Warn(b.logger).Log("msg", "truncateMessage: Unable to find the end of last alert.")
		}
		return truncateMsg
	}
	return truncateMsg
}

func parseMuteCommand(text string, environments []string, projects []string) ([]string, []string ,error) {
	matchProjectAndEnvironment, err := regexp.MatchString(ProjectAndEnvironmentRegexp, text)
	if err != nil {
		return []string{}, []string{}, err
	}

	regexProject, err := regexp.Compile(ProjectValuesRegexp)
	regexEnvironment, err := regexp.Compile(EnvironmentValuesRegexp)

	if matchProjectAndEnvironment {
		env := strings.Replace(regexEnvironment.FindStringSubmatch(text)[1], " ", "", -1)
		environmentsToMute := strings.Split(env, ",")

		p := strings.Replace(regexProject.FindStringSubmatch(text)[1], " ", "", -1)
		projectsToMute := strings.Split(p, ",")
		return arrayDifference(environments, environmentsToMute), arrayDifference(projects, projectsToMute), nil
	}

	matchEnvironment, err := regexp.MatchString(EnvironmentRegexp, text)
	if matchEnvironment {
		env := strings.Replace(regexEnvironment.FindStringSubmatch(text)[1], " ", "", -1)
		environmentsToMute := strings.Split(env, ",")
		return arrayDifference(environments, environmentsToMute), []string{}, nil
	}

	matchProject, err := regexp.MatchString(ProjectRegexp, text)
	if matchProject {
		p := strings.Replace(regexProject.FindStringSubmatch(text)[1], " ", "", -1)
		projectsToRemove := strings.Split(p, ",")
		return []string{}, arrayDifference(projects, projectsToRemove), nil
	}

	return []string{}, []string{}, errors.New("No match were found")
}

func arrayDifference(a, b []string) []string {
	mb := make(map[string]struct{}, len(b))
	for _, x := range b {
		mb[x] = struct{}{}
	}
	var diff []string
	for _, x := range a {
		if _, found := mb[x]; !found {
			diff = append(diff, x)
		}
	}
	return diff
}

func getUniqueChats(chats []telebot.Chat) []telebot.Chat {
	uniqueSet := make(map[telebot.Chat]bool, len(chats))
	for _, x := range chats {
		uniqueSet[x] = true
	}
	uniqueChats := make([]telebot.Chat, 0, len(uniqueSet))
	for x := range uniqueSet {
		uniqueChats = append(uniqueChats, x)
	}
	return uniqueChats
}
