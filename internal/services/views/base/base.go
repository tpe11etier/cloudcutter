package base

import (
	"context"
	"github.com/rivo/tview"
	"sync"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/tpelletiersophos/ahtui/ui/components"
)

type AWSService interface {
	InitWithConfig(aws.Config) error
}

type BaseServiceView struct {
	name        string
	LeftPanel   *components.LeftPanel
	MainContent tview.Primitive
	DataTable   *components.DataTable
	Layout      *tview.Flex

	awsConfig aws.Config

	Ctx             context.Context
	cancelFunc      context.CancelFunc
	CurrentOpCancel context.CancelFunc
	mu              sync.Mutex
	AWSServices     map[string]AWSService
}

func NewBaseServiceView(name string, awsConfig aws.Config) *BaseServiceView {
	base := &BaseServiceView{
		name:        name,
		awsConfig:   awsConfig,
		AWSServices: make(map[string]AWSService),
	}
	return base
}
