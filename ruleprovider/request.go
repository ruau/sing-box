package ruleprovider

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"time"

	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/ruleprovider/clash"
)

func request(ctx context.Context, logger log.ContextLogger, httpClient *http.Client, format Format, behavior Behavior, url string) (*Cache, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req = req.WithContext(ctx)
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	buffer := bytes.NewBuffer(nil)
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		resp.Body.Close()
		return nil, err
	}
	resp.Body.Close()

	isText := false
	switch format {
	case "", FormatYAML:
	case FormatText:
		isText = true
	}

	var set *clash.ClashRuleSet
	switch behavior {
	case BehaviorDomain:
		set, err = clash.ParseDomain(logger, buffer.Bytes(), isText)
	case BehaviorIPCIDR:
		set, err = clash.ParseIPCIDR(logger, buffer.Bytes(), isText)
	case BehaviorClassical:
		set, err = clash.ParseClassical(logger, buffer.Bytes())
	}
	if err != nil {
		return nil, err
	}

	cache := &Cache{
		RuleSet:    set,
		RuleCount:  uint64(set.GetCount()),
		Behavior:   string(behavior),
		Format:     string(format),
		LastUpdate: time.Now(),
	}
	return cache, nil
}
