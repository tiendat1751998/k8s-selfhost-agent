package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/chromedp/cdproto/runtime"
	"github.com/chromedp/chromedp"
)

func main() {
	// 1. Create a context for chromedp
	opts := append(chromedp.DefaultExecAllocatorOptions[:],
		chromedp.DisableGPU,
		chromedp.NoSandbox,
		chromedp.Headless,
	)
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), opts...)
	defer cancelAlloc()

	ctx, cancelCtx := chromedp.NewContext(allocCtx)
	defer cancelCtx()

	// Set a timeout for the entire testing process
	ctx, cancelTimeout := context.WithTimeout(ctx, 30*time.Second)
	defer cancelTimeout()

	// Capture console log events
	var consoleErrors []string
	chromedp.ListenTarget(ctx, func(ev interface{}) {
		switch ev := ev.(type) {
		case *runtime.EventConsoleAPICalled:
			if ev.Type == runtime.APITypeError {
				for _, arg := range ev.Args {
					consoleErrors = append(consoleErrors, fmt.Sprintf("Console Error: %s", arg.Value))
				}
			}
		case *runtime.EventExceptionThrown:
			consoleErrors = append(consoleErrors, fmt.Sprintf("JS Exception: %s", ev.ExceptionDetails.Text))
		}
	})

	// Ensure output directory for screenshots exists
	outputDir := filepath.Join("C:", "Users", "datdt", ".gemini", "antigravity", "brain", "05612643-dd90-453c-b4db-aed6fc477f4e", "screenshots")
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		log.Fatalf("Failed to create screenshots directory: %v", err)
	}

	fmt.Println("🚀 Starting automated browser test...")
	fmt.Println("1. Navigating to login page...")

	var homeBuf, complianceBuf, costBuf, obsBuf, topologyBuf []byte

	err := chromedp.Run(ctx,
		// Navigate to host
		chromedp.Navigate("http://localhost:8080"),
		// Wait for email input to appear (Login Modal is visible)
		chromedp.WaitVisible(`input[type="email"]`, chromedp.ByQuery),
		chromedp.Sleep(500*time.Millisecond),

		// Fill in credentials
		chromedp.SendKeys(`input[type="email"]`, "admin@k8sselfhost.local", chromedp.ByQuery),
		chromedp.SendKeys(`input[type="password"]`, "admin", chromedp.ByQuery),
		chromedp.Click(`#login-form button[type="submit"]`, chromedp.ByQuery),
		
		// Wait for redirect, reload, and overview section load
		chromedp.Sleep(2*time.Second),
		chromedp.WaitVisible(`a[data-section="overview"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),

		// 2. Capture Overview/Home Page
		chromedp.ActionFunc(func(ctx context.Context) error {
			fmt.Println("2. Logged in successfully! Capturing Home overview...")
			return nil
		}),
		chromedp.Screenshot(`body`, &homeBuf, chromedp.ByQuery),

		// 3. Navigate to Compliance tab and capture
		chromedp.ActionFunc(func(ctx context.Context) error {
			fmt.Println("3. Navigating to Compliance tab...")
			return nil
		}),
		chromedp.Click(`a[data-section="compliance"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Screenshot(`body`, &complianceBuf, chromedp.ByQuery),

		// 4. Navigate to Cost Management tab and capture
		chromedp.ActionFunc(func(ctx context.Context) error {
			fmt.Println("4. Navigating to Cost Management tab...")
			return nil
		}),
		chromedp.Click(`a[data-section="cost-management"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Screenshot(`body`, &costBuf, chromedp.ByQuery),

		// 5. Navigate to Observability tab and capture
		chromedp.ActionFunc(func(ctx context.Context) error {
			fmt.Println("5. Navigating to Observability tab...")
			return nil
		}),
		chromedp.Click(`a[data-section="observability"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Screenshot(`body`, &obsBuf, chromedp.ByQuery),

		// 6. Navigate to Topology tab and capture
		chromedp.ActionFunc(func(ctx context.Context) error {
			fmt.Println("6. Navigating to Topology tab...")
			return nil
		}),
		chromedp.Click(`a[data-section="topology"]`, chromedp.ByQuery),
		chromedp.Sleep(1*time.Second),
		chromedp.Screenshot(`body`, &topologyBuf, chromedp.ByQuery),
	)

	if err != nil {
		log.Fatalf("Browser automation failed: %v", err)
	}

	// Write screenshots to disk
	_ = os.WriteFile(filepath.Join(outputDir, "01_home.png"), homeBuf, 0644)
	_ = os.WriteFile(filepath.Join(outputDir, "02_compliance.png"), complianceBuf, 0644)
	_ = os.WriteFile(filepath.Join(outputDir, "03_cost.png"), costBuf, 0644)
	_ = os.WriteFile(filepath.Join(outputDir, "04_observability.png"), obsBuf, 0644)
	_ = os.WriteFile(filepath.Join(outputDir, "05_topology.png"), topologyBuf, 0644)

	fmt.Println("✅ Browser testing finished successfully! Screenshots saved at:", outputDir)

	if len(consoleErrors) > 0 {
		fmt.Println("\n⚠️ Detected the following console errors/exceptions during test:")
		for _, e := range consoleErrors {
			fmt.Println("- ", e)
		}
	} else {
		fmt.Println("\n🎉 No Javascript console errors or exceptions detected!")
	}
}
