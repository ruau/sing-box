//go:build with_outbound_provider

package outbound

import (
	"bytes"
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/sagernet/quic-go"
	"github.com/sagernet/quic-go/http3"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/common/dialer"
	"github.com/sagernet/sing-box/common/outboundprovider/action"
	"github.com/sagernet/sing-box/common/outboundprovider/proxyparser"
	"github.com/sagernet/sing-box/common/taskmonitor"
	C "github.com/sagernet/sing-box/constant"
	"github.com/sagernet/sing-box/log"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing/common"
	"github.com/sagernet/sing/common/bufio"
	E "github.com/sagernet/sing/common/exceptions"
	F "github.com/sagernet/sing/common/format"
	M "github.com/sagernet/sing/common/metadata"
	N "github.com/sagernet/sing/common/network"
	"github.com/sagernet/sing/service"
)

const (
	defaultRequestTimeout = 30 * time.Second
)

var (
	defaultUserAgent = ""
	regTraffic       *regexp.Regexp
	regExpire        *regexp.Regexp
)

func init() {
	defaultUserAgent = fmt.Sprintf(
		"clash; clash-meta; meta; sing/%s; sing-box/%s; SFA/%s; SFI/%s; SFT/%s; SFM/%s",
		C.Version,
		C.Version,
		C.Version,
		C.Version,
		C.Version,
		C.Version,
	)
	regTraffic = regexp.MustCompile(`upload=(\d+); download=(\d+); total=(\d+)`)
	regExpire = regexp.MustCompile(`expire=(\d+)`)
}

var (
	_ adapter.Outbound         = (*Provider)(nil)
	_ adapter.OutboundGroup    = (*Provider)(nil)
	_ adapter.OutboundProvider = (*Provider)(nil)
)

type Provider struct {
	ctx              context.Context
	router           adapter.Router
	logFactory       log.Factory
	logger           log.ContextLogger
	tag              string
	url              string
	cache_tag        string
	http3            bool
	updateInterval   time.Duration
	requestTimeout   time.Duration
	headers          http.Header
	dialer           N.Dialer
	dialerDetour     string
	httpClient       *http.Client
	httpTransport    http.RoundTripper
	selectorOptions  option.SelectorOutboundOptions
	actionGroup      *action.ProviderActionGroup
	updateLocker     sync.Mutex
	locker           sync.RWMutex
	outbounds        []adapter.Outbound
	outboundMap      map[string]adapter.Outbound
	groupOutbounds   []adapter.OutboundGroup
	groupOutboundMap map[string]adapter.OutboundGroup
	globalOutbound   *Selector
	providerInfo     *adapter.OutboundProviderInfo
	loopUpdateCancel context.CancelFunc
	startOnce        sync.Once
	startErr         error
}

func NewProvider(ctx context.Context, router adapter.Router, logFactory log.Factory, logger log.ContextLogger, tag string, options option.ProviderOutboundOptions) (*Provider, error) {
	outbound := &Provider{
		ctx:        ctx,
		router:     router,
		logFactory: logFactory,
		logger:     logger,
		tag:        tag,
		cache_tag:  tag,
	}
	if options.URL == "" {
		return nil, E.New("missing url")
	}
	outbound.url = options.URL
	if options.CacheTag != "" {
		outbound.cache_tag = options.CacheTag
	}
	outbound.http3 = options.HTTP3
	if options.UpdateInterval > 0 {
		outbound.updateInterval = time.Duration(options.UpdateInterval)
	}
	if options.RequestTimeout > 0 {
		outbound.requestTimeout = time.Duration(options.RequestTimeout)
	} else {
		outbound.requestTimeout = defaultRequestTimeout
	}
	d, err := dialer.New(router, options.ProviderDialer)
	if err != nil {
		return nil, err
	}
	outbound.dialer = d
	outbound.dialerDetour = options.ProviderDialer.Detour
	outbound.headers = make(http.Header)
	outbound.headers.Set("User-Agent", defaultUserAgent)
	for k, v := range options.Headers {
		outbound.headers.Set(k, v)
	}
	outbound.selectorOptions = options.SelectorOptions
	if len(options.Actions) > 0 {
		group, err := action.NewProviderActionGroup(options.Actions)
		if err != nil {
			return nil, err
		}
		outbound.actionGroup = group
	}
	err = router.RegisterOutboundProvider(tag, outbound)
	if err != nil {
		return nil, err
	}
	return outbound, nil
}

func (p *Provider) requestHTTP(ctx context.Context) (http.Header, []byte, error) {
	if p.requestTimeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, p.requestTimeout)
		defer cancel()
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, p.url, nil)
	if err != nil {
		return nil, nil, err
	}
	httpClient := p.httpClient
	if httpClient == nil {
		httpClient = &http.Client{}
		if !p.http3 {
			tr := &http.Transport{
				DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
					return p.dialer.DialContext(ctx, network, M.ParseSocksaddr(addr))
				},
				ForceAttemptHTTP2: true,
			}
			httpClient.Transport = tr
			p.httpTransport = tr
		} else {
			tr := &http3.RoundTripper{
				Dial: func(ctx context.Context, addr string, tlsCfg *tls.Config, cfg *quic.Config) (quic.EarlyConnection, error) {
					conn, dialErr := p.dialer.DialContext(ctx, N.NetworkUDP, M.ParseSocksaddr(addr))
					if dialErr != nil {
						return nil, dialErr
					}
					return quic.DialEarly(ctx, bufio.NewUnbindPacketConn(conn), conn.RemoteAddr(), tlsCfg, cfg)
				},
			}
			httpClient.Transport = tr
			p.httpTransport = tr
		}
	}
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()
	buffer := bytes.NewBuffer(nil)
	_, err = io.Copy(buffer, resp.Body)
	if err != nil {
		return nil, nil, err
	}
	return resp.Header, buffer.Bytes(), nil
}

func (p *Provider) fetch(ctx context.Context) (*adapter.OutboundProviderInfo, error) {
	headers, data, err := p.requestHTTP(ctx)
	if err != nil {
		return nil, err
	}
	outbounds, err := proxyparser.ParseOutbound(data)
	if err != nil {
		return nil, err
	}
	info := &adapter.OutboundProviderInfo{
		LastUpdated: time.Now(),
		Outbounds:   outbounds,
	}
	subscriptionUserInfo := headers.Get("subscription-userinfo")
	if subscriptionUserInfo != "" {
		subscriptionUserInfo = strings.ToLower(subscriptionUserInfo)
		matchTraffic := regTraffic.FindStringSubmatch(subscriptionUserInfo)
		if len(matchTraffic) == 4 {
			uploadUint64, err := strconv.ParseUint(matchTraffic[1], 10, 64)
			if err == nil {
				info.Upload = uploadUint64
			}
			downloadUint64, err := strconv.ParseUint(matchTraffic[2], 10, 64)
			if err == nil {
				info.Download = downloadUint64
			}
			totalUint64, err := strconv.ParseUint(matchTraffic[3], 10, 64)
			if err == nil {
				info.Total = totalUint64
			}
		}
		matchExpire := regExpire.FindStringSubmatch(subscriptionUserInfo)
		if len(matchExpire) == 2 {
			expireUint64, err := strconv.ParseUint(matchExpire[1], 10, 64)
			if err == nil {
				info.Expired = time.Unix(int64(expireUint64), 0)
			}
		}
	}
	return info, nil
}

func (p *Provider) loadOrfetchInfo(ctx context.Context) (*adapter.OutboundProviderInfo, error) {
	cacheFile := service.FromContext[adapter.CacheFile](p.ctx)
	if cacheFile != nil {
		info := cacheFile.LoadOutboundProviderInfo(p.cache_tag)
		if info != nil && (p.updateInterval == 0 || time.Since(info.LastUpdated) < p.updateInterval) {
			return info, nil
		}
	}
	info, err := p.fetch(ctx)
	if err != nil {
		return nil, err
	}
	if cacheFile != nil {
		cacheFile.SaveOutboundProviderInfo(p.cache_tag, info)
	}
	return info, nil
}

func (p *Provider) update(ctx context.Context) error {
	p.updateLocker.Lock()
	defer p.updateLocker.Unlock()
	info, err := p.loadOrfetchInfo(ctx)
	if err != nil {
		p.logger.Error("failed to update outbound info: ", err)
	} else {
		p.logger.Info("outbound info updated")
	}
	info.Outbounds = nil
	p.providerInfo = info
	return err
}

func (p *Provider) loopUpdate(ctx context.Context, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			break
		case <-ticker.C:
			p.update(ctx)
		}
	}
}

func (p *Provider) Update(ctx context.Context) error {
	if ctx == nil {
		ctx = p.ctx
	}
	return p.update(ctx)
}

func (p *Provider) ProviderInfo() *adapter.OutboundProviderInfo {
	return p.providerInfo
}

func (p *Provider) start() error {
	info, err := p.loadOrfetchInfo(p.ctx)
	if err != nil {
		return err
	}
	if len(info.Outbounds) == 0 {
		return E.New("missing outbound")
	}
	p.logger.Debug("outbound info loaded")

	outboundPtrs := make([]*option.Outbound, 0, len(info.Outbounds))
	for i := range info.Outbounds {
		outboundPtrs = append(outboundPtrs, &info.Outbounds[i])
	}
	var (
		outboundOptions      []*option.Outbound
		groupOutboundOptions []*option.Outbound
	)
	if p.actionGroup != nil {
		p.logger.Debug("execute outbound actions")
		groupContext, err := p.actionGroup.Execute(p.ctx, p.router, p.logger, outboundPtrs)
		if err != nil {
			return err
		}
		p.logger.Debug("outbound actions executed")
		outboundOptions = groupContext.Outbounds()
		groupOutboundOptions = groupContext.GroupOutbounds()
	} else {
		outboundOptions = outboundPtrs
	}

	// Create Outbound
	globalOutboundTags := make([]string, 0, len(outboundOptions)+len(groupOutboundOptions))
	p.outbounds = make([]adapter.Outbound, 0, len(outboundOptions))
	p.outboundMap = make(map[string]adapter.Outbound)
	for i, opt := range outboundOptions {
		out, err := New(p.ctx, p.router, p.logFactory, p.logFactory.NewLogger(F.ToString("outbound/", opt.Type, "[", opt.Tag, "]")), opt.Tag, *opt)
		if err != nil {
			return E.Cause(err, "parse outbound[", i, "] [", opt.Tag, "]")
		}
		p.outbounds = append(p.outbounds, out)
		p.outboundMap[out.Tag()] = out
		globalOutboundTags = append(globalOutboundTags, out.Tag())
	}
	if len(groupOutboundOptions) > 0 {
		p.groupOutbounds = make([]adapter.OutboundGroup, 0, len(groupOutboundOptions))
		p.groupOutboundMap = make(map[string]adapter.OutboundGroup)
		for i, opt := range groupOutboundOptions {
			out, err := New(p.ctx, p.router, p.logFactory, p.logFactory.NewLogger(F.ToString("outbound/", opt.Type, "[", opt.Tag, "]")), opt.Tag, *opt)
			if err != nil {
				return E.Cause(err, "parse group outbound[", i, "] [", opt.Tag, "]")
			}
			p.groupOutbounds = append(p.groupOutbounds, out.(adapter.OutboundGroup))
			p.groupOutboundMap[out.Tag()] = out.(adapter.OutboundGroup)
			globalOutboundTags = append(globalOutboundTags, out.Tag())
		}
	}
	globalOutboundOptions := p.selectorOptions
	globalOutboundOptions.Outbounds = append(globalOutboundOptions.Outbounds, globalOutboundTags...)
	p.globalOutbound, err = NewSelector(p.ctx, p.router, p.logger, p.tag, globalOutboundOptions)
	if err != nil {
		return E.Cause(err, "parse global outbound")
	}
	err = p.router.CheckOutboundProvider(p.tag)
	if err != nil {
		return err
	}

	// Start Outbound
	failedTag := false
	monitor := taskmonitor.New(p.logger, C.StartTimeout)
	for _, out := range p.outbounds {
		starter, isStarter := out.(common.Starter)
		if isStarter {
			monitor.Start("initialize outbound/", out.Type(), "[", out.Tag(), "]")
			err = starter.Start()
			monitor.Finish()
			if err != nil {
				failedTag = true
				return E.Cause(err, "initialize outbound/", out.Type(), "[", out.Tag(), "]")
			}
		}
		defer func(out adapter.Outbound) {
			if failedTag {
				common.Close(out)
			}
		}(out)
	}
	for _, out := range p.groupOutbounds {
		starter, isStarter := out.(common.Starter)
		if isStarter {
			monitor.Start("initialize group outbound/", out.Type(), "[", out.Tag(), "]")
			err = starter.Start()
			monitor.Finish()
			if err != nil {
				failedTag = true
				return E.Cause(err, "initialize group outbound/", out.Type(), "[", out.Tag(), "]")
			}
		}
		p.groupOutboundMap[out.Tag()] = out
		defer func(out adapter.OutboundGroup) {
			if failedTag {
				common.Close(out)
			}
		}(out)
	}
	err = p.globalOutbound.Start()
	if err != nil {
		failedTag = true
		return E.Cause(err, "initialize global outbound")
	}

	info.Outbounds = nil
	p.providerInfo = info

	return nil
}

func (p *Provider) lazyStart() error {
	p.startOnce.Do(func() {
		p.startErr = p.start()
	})
	return p.startErr
}

func (p *Provider) Start() error {
	return p.lazyStart()
}

func (p *Provider) PostStart() error {
	// Outbounds PostStart
	var err error
	for _, out := range p.outbounds {
		postStarter, isPostStarter := out.(adapter.PostStarter)
		if isPostStarter {
			err = postStarter.PostStart()
			if err != nil {
				return E.Cause(err, "post-start outbound/", out.Type(), "[", out.Tag(), "]")
			}
		}
	}
	for _, out := range p.groupOutbounds {
		postStarter, isPostStarter := out.(adapter.PostStarter)
		if isPostStarter {
			err = postStarter.PostStart()
			if err != nil {
				return E.Cause(err, "post-start group outbound/", out.Type(), "[", out.Tag(), "]")
			}
		}
	}

	if p.updateInterval > 0 {
		var loopUpdateCtx context.Context
		loopUpdateCtx, p.loopUpdateCancel = context.WithCancel(p.ctx)
		go p.loopUpdate(loopUpdateCtx, p.updateInterval)
	}

	return nil
}

func (p *Provider) Close() error {
	if p.loopUpdateCancel != nil {
		p.loopUpdateCancel()
		p.loopUpdateCancel = nil
	}
	httpTr, ok := p.httpTransport.(*http.Transport)
	if ok {
		httpTr.CloseIdleConnections()
	}
	http3Tr, ok := p.httpTransport.(*http3.RoundTripper)
	if ok {
		http3Tr.Close()
	}

	// Close Outbounds
	common.Close(p.groupOutbounds)
	common.Close(p.outbounds)

	return nil
}

// Outbound

func (p *Provider) Tag() string {
	return p.tag
}

func (p *Provider) Type() string {
	return C.TypeProvider
}

func (p *Provider) Network() []string {
	if p.globalOutbound != nil {
		return p.globalOutbound.Network()
	}
	return []string{N.NetworkTCP, N.NetworkUDP}
}

func (p *Provider) Dependencies() []string {
	if p.dialerDetour != "" {
		return []string{p.dialerDetour}
	} else {
		return []string{}
	}
}

func (p *Provider) DialContext(ctx context.Context, network string, address M.Socksaddr) (net.Conn, error) {
	err := p.lazyStart()
	if err != nil {
		return nil, E.Cause(err, "failed to lazy start")
	}
	return p.globalOutbound.DialContext(ctx, network, address)
}

func (p *Provider) ListenPacket(ctx context.Context, address M.Socksaddr) (net.PacketConn, error) {
	err := p.lazyStart()
	if err != nil {
		return nil, E.Cause(err, "failed to lazy start")
	}
	return p.globalOutbound.ListenPacket(ctx, address)
}

func (p *Provider) NewConnection(ctx context.Context, conn net.Conn, metadata adapter.InboundContext) error {
	err := p.lazyStart()
	if err != nil {
		return E.Cause(err, "failed to lazy start")
	}
	return p.globalOutbound.NewConnection(ctx, conn, metadata)
}

func (p *Provider) NewPacketConnection(ctx context.Context, conn N.PacketConn, metadata adapter.InboundContext) error {
	err := p.lazyStart()
	if err != nil {
		return E.Cause(err, "failed to lazy start")
	}
	return p.globalOutbound.NewPacketConnection(ctx, conn, metadata)
}

func (p *Provider) InterfaceUpdated() {
	for _, outbound := range p.outbounds {
		listener, ok := outbound.(adapter.InterfaceUpdateListener)
		if ok {
			listener.InterfaceUpdated()
		}
	}
	for _, outbound := range p.groupOutbounds {
		listener, ok := outbound.(adapter.InterfaceUpdateListener)
		if ok {
			listener.InterfaceUpdated()
		}
	}
}

// OutboundGroup

func (p *Provider) All() []string {
	err := p.lazyStart()
	if err != nil {
		return []string{}
	}
	return p.globalOutbound.All()
}

func (p *Provider) Now() string {
	err := p.lazyStart()
	if err != nil {
		return ""
	}
	return p.globalOutbound.Now()
}

// OutboundProvider

func (p *Provider) Outbound(tag string) (adapter.Outbound, bool) {
	outbound, loaded := p.outboundMap[tag]
	if !loaded && p.groupOutboundMap != nil && len(p.groupOutboundMap) > 0 {
		outbound, loaded = p.groupOutboundMap[tag]
	}
	return outbound, loaded
}

func (p *Provider) BasicOutbounds() []adapter.Outbound {
	return p.outbounds
}

func (p *Provider) GroupOutbounds() []adapter.OutboundGroup {
	return p.groupOutbounds
}
