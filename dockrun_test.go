package dockrun_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/mafredri/cdp"
	"github.com/mafredri/cdp/cdpcmd"
	"github.com/mafredri/cdp/devtool"
	"github.com/mafredri/cdp/rpcc"
	dockrun "github.com/matthewmueller/go-dockrun"
	"github.com/stretchr/testify/assert"
)

func TestContainer(t *testing.T) {
	client, err := dockrun.New()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	container, err := client.
		Container("yukinying/chrome-headless:latest", "chromium").
		Expose("9222:9222").
		Run(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer container.Kill()

	err = container.Wait(ctx, "http://localhost:9222")
	if err != nil {
		t.Fatal(err)
	}

	// Use the DevTools json API to get the current page.
	devt := devtool.New("http://127.0.0.1:9222")
	page, err := devt.Get(ctx, devtool.Page)
	if err != nil {
		fmt.Println(err)
		page, err = devt.Create(ctx)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Connect to Chrome Debugging Protocol target.
	conn, err := rpcc.DialContext(ctx, page.WebSocketDebuggerURL)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close() // Must be closed when we are done.

	// Create a new CDP Client that uses conn.
	c := cdp.NewClient(conn)

	// Enable events on the Page domain.
	if err = c.Page.Enable(ctx); err != nil {
		t.Fatal(err)
	}

	// New DOMContentEventFired client will receive and buffer
	// ContentEventFired events from now on.
	domContentEventFired, err := c.Page.DOMContentEventFired(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer domContentEventFired.Close()

	// Create the Navigate arguments with the optional Referrer field set.
	navArgs := cdpcmd.NewPageNavigateArgs("https://www.google.com")
	_, err = c.Page.Navigate(ctx, navArgs)
	if err != nil {
		t.Fatal(err)
	}

	// Block until a DOM ContentEventFired event is triggered.
	if _, err = domContentEventFired.Recv(); err != nil {
		t.Fatal(err)
	}

	evalArgs := cdpcmd.NewRuntimeEvaluateArgs("document.title")
	reply, err := c.Runtime.Evaluate(ctx, evalArgs)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, "\"Google\"", string(reply.Result.Value))
}
