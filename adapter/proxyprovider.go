package adapter

import (
	"time"

	"github.com/sagernet/sing-box/option"
)

type ProxyProvider interface {
	Service
	Tag() string
	StartGetOutbounds() ([]option.Outbound, error)
	GetOutboundOptions() ([]option.Outbound, error)
	GetFullOutboundOptions() ([]option.Outbound, error)
	GetClashInfo() (uint64, uint64, uint64, time.Time, error) // download, upload, total, expire, error
	LastUpdateTime() time.Time
	Update()
}
