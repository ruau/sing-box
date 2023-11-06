//go:build with_ruleprovider

package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/sagernet/sing-box/common/json"
	"github.com/sagernet/sing-box/option"
	"github.com/sagernet/sing-box/ruleprovider"

	"github.com/spf13/cobra"
)

var commandShowRule = &cobra.Command{
	Use:   "showrule",
	Short: "Show rule from clash rule",
	Run: func(cmd *cobra.Command, args []string) {
		showRule()
	},
}

var (
	ruleFile     string
	ruleLink     string
	ruleBehavior string
	ruleFormat   string
	ruleTag      string
)

func init() {
	commandShowRule.PersistentFlags().StringVarP(&ruleFile, "file", "f", "", "rule file")
	commandShowRule.PersistentFlags().StringVarP(&ruleLink, "link", "l", "", "rule link")
	commandShowRule.PersistentFlags().StringVarP(&ruleBehavior, "behavior", "b", string(ruleprovider.BehaviorDomain), "rule behavior (domain/ipcidr/classical)")
	commandShowRule.PersistentFlags().StringVarP(&ruleFormat, "format", "t", string(ruleprovider.FormatYAML), "rule format (text/yaml)")
	commandShowRule.PersistentFlags().StringVarP(&ruleTag, "tag", "g", "", "rule tag")
	mainCommand.AddCommand(commandShowRule)
}

func showRule() {
	if ruleLink == "" {
		fmt.Println("link is empty")
		os.Exit(1)
		return
	}

	var format ruleprovider.Format
	switch ruleFormat {
	case string(ruleprovider.FormatYAML), "":
		format = ruleprovider.FormatYAML
	case string(ruleprovider.FormatText):
		format = ruleprovider.FormatText
	default:
		fmt.Println("invalid format: ", ruleFormat)
		os.Exit(1)
		return
	}

	var behavior ruleprovider.Behavior
	switch ruleBehavior {
	case string(ruleprovider.BehaviorDomain):
		behavior = ruleprovider.BehaviorDomain
	case string(ruleprovider.BehaviorIPCIDR):
		behavior = ruleprovider.BehaviorIPCIDR
	case string(ruleprovider.BehaviorClassical):
		behavior = ruleprovider.BehaviorClassical
	default:
		fmt.Println("invalid behavior: ", ruleBehavior)
		os.Exit(1)
		return
	}

	var options option.Options
	raw, err := os.ReadFile(ruleFile)
	if err != nil {
		fmt.Println("read file error: ", err)
		os.Exit(1)
		return
	}
	err = options.UnmarshalJSON(raw)
	if err != nil {
		fmt.Println("unmarshal json error: ", err)
		os.Exit(1)
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		signalChan := make(chan os.Signal, 1)
		signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
		select {
		case <-signalChan:
			cancel()
		case <-ctx.Done():
			return
		}
	}()

	var (
		rawDNSRules   *[]option.DNSRule
		rawRouteRules *[]option.Rule
	)
	if options.DNS != nil && options.DNS.Rules != nil {
		rawDNSRules = &options.DNS.Rules
	}
	if options.Route != nil && options.Route.Rules != nil {
		rawRouteRules = &options.Route.Rules
	}
	dnsRules, routeRules, err := ruleprovider.ParseLink(ctx, ruleLink, ruleTag, format, behavior, rawDNSRules, rawRouteRules)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
	if options.DNS != nil && options.DNS.Rules != nil {
		options.DNS.Rules = dnsRules
	}
	if options.Route != nil && options.Route.Rules != nil {
		options.Route.Rules = routeRules
	}

	buffer := bytes.NewBuffer(nil)
	encoder := json.NewEncoder(buffer)
	encoder.SetIndent("", "  ")
	err = encoder.Encode(options)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	fmt.Println(buffer.String())
}
