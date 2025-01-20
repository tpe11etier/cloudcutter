package elastic

import "github.com/tpelletiersophos/cloudcutter/internal/ui/components/spinner"

func (v *View) showLoading(message string) {
	v.state.mu.Lock()
	defer v.state.mu.Unlock()

	if v.state.misc.spinner == nil {
		v.state.misc.spinner = spinner.NewSpinner(message)
		v.state.misc.spinner.SetOnComplete(func() {
			v.state.mu.Lock()
			defer v.state.mu.Unlock()
			pages := v.manager.Pages()
			pages.RemovePage("loading")
		})
	} else {
		v.state.misc.spinner.SetMessage(message)
	}

	if !v.state.misc.spinner.IsLoading() {
		modal := spinner.CreateSpinnerModal(v.state.misc.spinner)
		pages := v.manager.Pages()
		pages.AddPage("loading", modal, true, true)
		v.state.misc.spinner.Start(v.manager.App())
	}
}

func (v *View) hideLoading() {
	if v.state.misc.spinner != nil {
		v.state.misc.spinner.Stop()
	}
}
