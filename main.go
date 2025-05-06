package main

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/Jinnrry/pmail/dto/parsemail"
	"github.com/Jinnrry/pmail/hooks/framework"
	"github.com/Jinnrry/pmail/models"
	"github.com/Jinnrry/pmail/utils/context"
	log "github.com/sirupsen/logrus"
)

const configPath = "./plugins/pmail_jez_webhook.json"
const pluginName = "JezWebhook"

type panelConfigMap struct {
	Address       string `json:"address"`
	MaxRetries    int    `json:"max-retries"`
	InfiniteRetry bool   `json:"infinite-retry"`
	Enabled       bool   `json:"enabled"`
}
type config struct {
	Panels     []panelConfigMap `json:"panels"`
	AllowRetry bool             `json:"allow-retry"`
}

type plugin struct {
	config     *config
	httpClient *http.Client
}

func (plugin *plugin) GetName(ctx *context.Context) string {
	return pluginName
}

//go:embed static/index.html
var index string

func (plugin *plugin) SettingsHtml(ctx *context.Context, url string, requestData string) string {
	if strings.Contains(url, "index.html") {
		if !ctx.IsAdmin {
			return "<div>Please contact the administrator for configuration.</div>"
		}
		var builder strings.Builder
		for count, panel := range plugin.config.Panels {
			var infiniteRetryChecked string
			if panel.InfiniteRetry {
				infiniteRetryChecked = "checked"
			}
			var enabledChecked string
			if panel.Enabled {
				enabledChecked = "checked"
			}
			_, err := builder.WriteString(fmt.Sprintf(`
			<div class="config-panel" data-id="%d">
				<div class="form-group">
					<label for="address-%d">Address:</label>
					<input type="text" id="address-%d" placeholder="https://example.com:12345" value="%s">
				</div>
				<div class="form-group">
					<label for="port-%d">Max Retries:</label>
					<input type="number" id="max-retries-%d" min="0" value="%d">
				</div>
				<div class="form-group">
					<label for="https-%d">InfiniteRetry:</label>
					<input type="checkbox" id="infinite-retry-%d" %s>
				</div>
				<div class="form-group">
					<label for="enabled-%d">Enabled:</label>
					<input type="checkbox" id="enabled-%d" %s>
				</div>
				<button class="remove-btn" onclick="removeConfigPanel('%d')">Delete</button>
			</div>
			`, count, count, count, panel.Address, count, count, panel.MaxRetries, count, count, infiniteRetryChecked, count, count, enabledChecked, count))
			if err != nil {
				return fmt.Sprintf("<div>Error generating configuration HTML %s</div>", err)
			}
		}
		var allowRetryChecked string
		if plugin.config.AllowRetry {
			allowRetryChecked = "checked"
		}
		return fmt.Sprintf(index, builder.String(), allowRetryChecked)
	}
	var tmpConfig config
	err := json.Unmarshal([]byte(requestData), &tmpConfig)
	if err != nil {
		log.Error(err.Error())
		return err.Error()
	}
	plugin.config = &tmpConfig
	err = os.WriteFile(configPath, []byte(requestData), 0700)
	if err != nil {
		log.Error(err.Error())
		return err.Error()
	}
	return "success"
}

func (plugin *plugin) SendBefore(ctx *context.Context, email *parsemail.Email) {
}

func (plugin *plugin) SendAfter(ctx *context.Context, email *parsemail.Email, err map[string]error) {
}

func (plugin *plugin) ReceiveParseBefore(ctx *context.Context, email *[]byte) {
}

func (plugin *plugin) ReceiveParseAfter(ctx *context.Context, email *parsemail.Email) {
}

func (plugin *plugin) ReceiveSaveAfter(ctx *context.Context, email *parsemail.Email, ue []*models.UserEmail) {
	config := *plugin.config
	for _, panel := range config.Panels {
		if !panel.Enabled {
			return
		}
		address := panel.Address
		_, err := plugin.httpClient.Get(address)
		if err != nil {
			log.Warnf("%s : %s", address, err.Error())
			for (panel.InfiniteRetry || panel.MaxRetries > 0) && plugin.config.AllowRetry {
				time.Sleep(5 * time.Second)
				panel.MaxRetries--
				_, err := plugin.httpClient.Get(address)
				if err == nil {
					log.Infof("Success to Connect: %s", address)
					return
				}
				log.Warnf("%s : %s", address, err.Error())
			}
			log.Errorf("Fail to Connect: %s", address)
			return
		}
		log.Infof("Success to Connect: %s", address)
	}
}

func NewInstance() *plugin {
	var config config
	data, err := os.ReadFile(configPath)
	if err == nil {
		err = json.Unmarshal(data, &config)
		if err != nil {
			panic(err)
		}
	} else if os.IsExist(err) {
		panic(err)
	}

	result := &plugin{
		config: &config,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
	return result
}

func main() {
	framework.CreatePlugin(pluginName, NewInstance()).Run()
}
