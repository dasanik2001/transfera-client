// internal/tui/menu.go — Interactive Terminal UI
//
// When the user types `transfera` without any subcommand, or double-clicks
// transfera.exe in Windows Explorer, this interactive menu is displayed.
//
// It provides a friendly, menu-driven interface for all Transfera features:
//   1. Upload a file
//   2. Download a file
//   3. Install to PATH
//   4. Help
//   5. Exit

package tui

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/dasanik2001/transfera-client/cli/internal/api"
	"github.com/dasanik2001/transfera-client/cli/internal/installer"
	"github.com/dasanik2001/transfera-client/cli/internal/progress"
	"github.com/dasanik2001/transfera-client/cli/internal/validation"
	"github.com/schollz/progressbar/v3"
)

// =========================================================================
// Color helpers — ANSI escape codes for terminal styling
// =========================================================================

const (
	colorReset   = "\033[0m"
	colorCyan    = "\033[36m"
	colorGreen   = "\033[32m"
	colorYellow  = "\033[33m"
	colorRed     = "\033[31m"
	colorBold    = "\033[1m"
	colorDim     = "\033[2m"
	colorWhite   = "\033[97m"
	colorMagenta = "\033[35m"
)

// =========================================================================
// RunMenu — entry point for the interactive TUI
// =========================================================================

// RunMenu displays the interactive terminal menu and handles user actions.
// apiURL and verbose are pointers to the global config from root.go.
func RunMenu(apiURL *string, verbose *bool) {
	reader := bufio.NewReader(os.Stdin)

	clearScreen()
	printBanner()

	for {
		printMenu(*apiURL)
		choice := prompt(reader, fmt.Sprintf("\n  %s▶ Choose an option (1-8):%s ", colorCyan, colorReset))

		switch strings.TrimSpace(choice) {
		case "1":
			handleUpload(reader, *apiURL, *verbose)
		case "2":
			handleDownload(reader, *apiURL, *verbose)
		case "3":
			handleRooms(reader, *apiURL, *verbose)
		case "4":
			handleHealth(*apiURL, *verbose)
		case "5":
			handleInstall(reader)
		case "6":
			handleChangeServer(reader, apiURL)
		case "7":
			handleHelp()
		case "8", "q", "Q", "exit", "quit":
			fmt.Printf("\n  %s👋 Goodbye!%s\n\n", colorCyan, colorReset)
			return
		default:
			fmt.Printf("\n  %s✗ Invalid option. Please enter 1-8.%s\n", colorRed, colorReset)
		}

		fmt.Println()
		promptContinue(reader)
	}
}

// =========================================================================
// Banner & Menu Display
// =========================================================================

func clearScreen() {
	fmt.Print("\033[2J\033[H")
}

func printBanner() {
	fmt.Println()
	fmt.Print(colorCyan, colorBold)
	fmt.Println("   ___________                          _____")
	fmt.Println("  |           |                        / ____|")
	fmt.Println("  |--   ------'_ __ __ _ _ __  ___  | |__ ___ _ __ __ _")
	fmt.Println("     |  |    | '__/ _' | '_ \\/ __|  |  __/ _ \\ '__/ _' |")
	fmt.Println("     |  |    | | | (_| | | | \\__ \\  | | |  __/ | | (_| |")
	fmt.Println("     |__|    |_|  \\__,_|_| |_|___/  |_|  \\___|_|  \\__,_|")
	fmt.Print(colorReset)
	fmt.Println()
	fmt.Printf("  %s%sSecure P2P File Sharing — Terminal Edition%s\n", colorDim, colorWhite, colorReset)
	fmt.Println()

	// PATH installation status
	if installer.IsInPath() && installer.IsInstalled() {
		if installer.IsUpdateAvailable() {
			fmt.Printf("  %s● Status:%s %s🔄 Update available%s — select option 5 to update binary in PATH\n",
				colorDim, colorReset, colorYellow, colorReset)
		} else {
			fmt.Printf("  %s● Status:%s %s✓ Installed globally%s — type %stransfera%s anywhere\n",
				colorDim, colorReset, colorGreen, colorReset, colorBold, colorReset)
		}
	} else {
		fmt.Printf("  %s● Status:%s %s⚠ Not installed to PATH%s — select option 5 to install\n",
			colorDim, colorReset, colorYellow, colorReset)
	}
	fmt.Println()
}

func printMenu(apiURL string) {
	var opt5Icon, opt5Text string
	if installer.IsInPath() && installer.IsInstalled() {
		if installer.IsUpdateAvailable() {
			opt5Icon = "🔄"
			opt5Text = "Update in PATH (new version)        "
		} else {
			opt5Icon = "⚡"
			opt5Text = "Reinstall / Update in PATH          "
		}
	} else {
		opt5Icon = "⚡"
		opt5Text = "Install to PATH                     "
	}

	fmt.Printf("  %s%s┌──────────────────────────────────────────────┐%s\n", colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %-43s%s%s│%s\n", colorCyan, colorBold, colorReset, "What would you like to do?", colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s├──────────────────────────────────────────────┤%s\n", colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s1.%s 📤  Upload a file                       %s%s│%s\n", colorCyan, colorBold, colorReset, colorGreen, colorReset, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s2.%s 📥  Download a file                     %s%s│%s\n", colorCyan, colorBold, colorReset, colorGreen, colorReset, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s3.%s 👥  Collaborative Rooms (Create/Join)    %s%s│%s\n", colorCyan, colorBold, colorReset, colorGreen, colorReset, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s4.%s 🩺  Server Health Check                 %s%s│%s\n", colorCyan, colorBold, colorReset, colorGreen, colorReset, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s5.%s %s  %s%s%s│%s\n", colorCyan, colorBold, colorReset, colorYellow, colorReset, opt5Icon, opt5Text, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s6.%s ⚙️   Change Server URL                   %s%s│%s\n", colorCyan, colorBold, colorReset, colorDim, colorReset, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s7.%s ❓  Help & CLI Reference                %s%s│%s\n", colorCyan, colorBold, colorReset, colorDim, colorReset, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s│%s   %s8.%s ❌  Exit                                %s%s│%s\n", colorCyan, colorBold, colorReset, colorRed, colorReset, colorCyan, colorBold, colorReset)
	fmt.Printf("  %s%s└──────────────────────────────────────────────┘%s\n", colorCyan, colorBold, colorReset)
}

// =========================================================================
// Menu Handlers
// =========================================================================

func handleUpload(reader *bufio.Reader, apiURL string, verbose bool) {
	fmt.Printf("\n  %s%s── Upload a File ──%s\n\n", colorCyan, colorBold, colorReset)

	// Prompt for file path (supports drag-and-drop: strips quotes)
	filePath := prompt(reader, fmt.Sprintf("  %sFile path (drag & drop or type):%s ", colorWhite, colorReset))
	filePath = cleanPath(filePath)

	if filePath == "" {
		fmt.Printf("  %s✗ No file specified.%s\n", colorRed, colorReset)
		return
	}

	// Validate file
	fileSize, err := validation.ValidateFile(filePath, validation.DefaultMaxUploadMB)
	if err != nil {
		fmt.Printf("  %s✗ %s%s\n", colorRed, err, colorReset)
		return
	}

	// Prompt for max downloads
	maxDlStr := prompt(reader, fmt.Sprintf("  %sMax downloads (1-100, default 1):%s ", colorWhite, colorReset))
	maxDownloads := 1
	if maxDlStr != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(maxDlStr)); err == nil && n >= 1 && n <= 100 {
			maxDownloads = n
		}
	}

	filename := filepath.Base(filePath)
	fmt.Printf("\n  File: %s%s%s (%s)\n", colorBold, filename, colorReset, validation.FormatFileSize(fileSize))
	fmt.Printf("  Max downloads: %d\n\n", maxDownloads)

	// Create progress bar and upload
	bar := progress.NewUploadBar(fileSize, filename)
	client := api.NewClient(apiURL, verbose)

	result, err := client.Upload(filePath, maxDownloads, func(bytesSent int64) {
		bar.Set64(bytesSent)
	})

	if err != nil {
		fmt.Printf("\n  %s✗ Upload failed: %s%s\n", colorRed, err, colorReset)
		return
	}

	bar.Finish()

	// Display invite code
	fmt.Printf("\n  %s✓ File ready to share!%s\n", colorGreen, colorReset)
	fmt.Printf("  %s┌────────────────────────────────────────%s\n", colorCyan, colorReset)
	fmt.Printf("  %s│%s  Invite code: %s%s%d%s\n", colorCyan, colorReset, colorBold, colorGreen, result.Port, colorReset)
	fmt.Printf("  %s│%s  Max downloads: %d\n", colorCyan, colorReset, result.MaxDownloads)
	fmt.Printf("  %s└────────────────────────────────────────%s\n", colorCyan, colorReset)
	fmt.Printf("\n  Share this code to download on another device:\n")
	fmt.Printf("    %stransfera download %d%s\n", colorBold, result.Port, colorReset)
}

func handleDownload(reader *bufio.Reader, apiURL string, verbose bool) {
	fmt.Printf("\n  %s%s── Download a File ──%s\n\n", colorCyan, colorBold, colorReset)

	// Prompt for invite code
	codeStr := prompt(reader, fmt.Sprintf("  %sInvite code:%s ", colorWhite, colorReset))
	codeStr = strings.TrimSpace(codeStr)

	port, err := strconv.Atoi(codeStr)
	if err != nil || port <= 0 || port > 65535 {
		fmt.Printf("  %s✗ Invalid invite code '%s'. Must be a number (1-65535).%s\n", colorRed, codeStr, colorReset)
		return
	}

	// Prompt for output directory
	outputDir := prompt(reader, fmt.Sprintf("  %sSave to directory (Enter = current dir):%s ", colorWhite, colorReset))
	outputDir = cleanPath(outputDir)

	fmt.Printf("\n  Downloading from invite code: %s%d%s\n\n", colorBold, port, colorReset)

	client := api.NewClient(apiURL, verbose)
	var pBar *progressbar.ProgressBar

	result, err := client.Download(port, outputDir, "", func(filename string, received, total int64) {
		if pBar == nil {
			displayName := filename
			if displayName == "" {
				displayName = "file"
			}
			if total > 0 {
				pBar = progress.NewDownloadBar(total, displayName)
			} else {
				pBar = progress.NewDownloadBar(-1, displayName)
			}
		}
		pBar.Set64(received)
	})

	if err != nil {
		fmt.Printf("\n  %s✗ Download failed: %s%s\n", colorRed, err, colorReset)
		return
	}

	if pBar != nil {
		pBar.Finish()
	}

	fmt.Printf("\n  %s✓ Downloaded: %s%s (%s)\n", colorGreen, result.Filename, colorReset,
		validation.FormatFileSize(result.FileSize))

	if result.DownloadsRemaining >= 0 {
		if result.DownloadsRemaining == 0 {
			fmt.Printf("    %sThis was the last allowed download. Invite code is now expired.%s\n", colorYellow, colorReset)
		} else {
			fmt.Printf("    Downloads remaining: %d\n", result.DownloadsRemaining)
		}
	}
	fmt.Printf("    Saved to: %s%s%s\n", colorBold, result.FilePath, colorReset)
}

func handleRooms(reader *bufio.Reader, apiURL string, verbose bool) {
	fmt.Printf("\n  %s%s── Collaborative Rooms ──%s\n\n", colorCyan, colorBold, colorReset)
	fmt.Printf("  %s1.%s ➕ Create a new room\n", colorGreen, colorReset)
	fmt.Printf("  %s2.%s 🔗 Join an existing room\n", colorGreen, colorReset)
	fmt.Printf("  %s3.%s 🔙 Back to main menu\n", colorDim, colorReset)

	choice := prompt(reader, fmt.Sprintf("\n  %s▶ Choose an option (1-3):%s ", colorCyan, colorReset))
	switch strings.TrimSpace(choice) {
	case "1":
		handleCreateRoom(reader, apiURL, verbose)
	case "2":
		handleJoinRoom(reader, apiURL, verbose)
	case "3", "b", "B", "back":
		return
	default:
		fmt.Printf("  %s✗ Invalid option.%s\n", colorRed, colorReset)
	}
}

func handleCreateRoom(reader *bufio.Reader, apiURL string, verbose bool) {
	fmt.Printf("\n  %s%s── Create a Collaborative Room ──%s\n\n", colorCyan, colorBold, colorReset)

	name := prompt(reader, fmt.Sprintf("  %sYour display name (Enter = Host):%s ", colorWhite, colorReset))
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Host"
	}

	capStr := prompt(reader, fmt.Sprintf("  %sMax capacity (2-100, default 5):%s ", colorWhite, colorReset))
	capacity := 5
	if capStr != "" {
		if n, err := strconv.Atoi(strings.TrimSpace(capStr)); err == nil && n >= 2 && n <= 100 {
			capacity = n
		}
	}

	portStr := prompt(reader, fmt.Sprintf("  %sCustom Room ID / Port (Enter = auto-assign):%s ", colorWhite, colorReset))
	customPort := 0
	if portStr != "" {
		if p, err := strconv.Atoi(strings.TrimSpace(portStr)); err == nil && p >= 1 && p <= 65535 {
			customPort = p
		}
	}

	client := api.NewClient(apiURL, verbose)
	fmt.Printf("\n  %sCreating collaborative room...%s\n", colorDim, colorReset)
	resp, err := client.CreateRoom(name, capacity, customPort)
	if err != nil {
		fmt.Printf("  %s✗ Failed to create room: %s%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n  %s✓ Room created successfully!%s\n", colorGreen, colorReset)
	fmt.Printf("  %s┌──────────────────────────────────────────────%s\n", colorCyan, colorReset)
	fmt.Printf("  %s│%s  Room ID (Port) : %s%s%d%s\n", colorCyan, colorReset, colorBold, colorGreen, resp.RoomID, colorReset)
	fmt.Printf("  %s│%s  Host           : %s\n", colorCyan, colorReset, name)
	fmt.Printf("  %s│%s  User ID        : %s%s%s\n", colorCyan, colorReset, colorDim, resp.UserID, colorReset)
	fmt.Printf("  %s│%s  Max Capacity   : %d participants\n", colorCyan, colorReset, resp.MaxParticipants)
	fmt.Printf("  %s└──────────────────────────────────────────────%s\n", colorCyan, colorReset)

	enter := prompt(reader, fmt.Sprintf("\n  %sEnter interactive room session now? (Y/n):%s ", colorWhite, colorReset))
	if strings.ToLower(strings.TrimSpace(enter)) != "n" {
		RunRoomSession(client, resp.RoomID, resp.UserID, name, true, resp.MaxParticipants)
	}
}

func handleJoinRoom(reader *bufio.Reader, apiURL string, verbose bool) {
	fmt.Printf("\n  %s%s── Join a Collaborative Room ──%s\n\n", colorCyan, colorBold, colorReset)

	roomStr := prompt(reader, fmt.Sprintf("  %sRoom ID (Port):%s ", colorWhite, colorReset))
	port, err := strconv.Atoi(strings.TrimSpace(roomStr))
	if err != nil || port <= 0 || port > 65535 {
		fmt.Printf("  %s✗ Invalid room ID '%s'. Must be a port number (1-65535).%s\n", colorRed, roomStr, colorReset)
		return
	}

	name := prompt(reader, fmt.Sprintf("  %sYour display name (Enter = Guest):%s ", colorWhite, colorReset))
	name = strings.TrimSpace(name)
	if name == "" {
		name = "Guest"
	}

	client := api.NewClient(apiURL, verbose)
	fmt.Printf("\n  %sJoining room %d...%s\n", colorDim, port, colorReset)
	resp, err := client.JoinRoom(port, name)
	if err != nil {
		fmt.Printf("  %s✗ Failed to join room: %s%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("\n  %s✓ Joined room %d successfully!%s\n", colorGreen, port, colorReset)
	RunRoomSession(client, resp.RoomID, resp.UserID, name, false, resp.MaxParticipants)
}

func handleHealth(apiURL string, verbose bool) {
	fmt.Printf("\n  %s%s── Server Health Check ──%s\n\n", colorCyan, colorBold, colorReset)

	isLocal := strings.Contains(apiURL, "127.0.0.1") || strings.Contains(apiURL, "localhost")
	if isLocal {
		fmt.Printf("  Checking API server at %s ...\n", apiURL)
	} else {
		fmt.Printf("  Checking API server at %s %s(may take a few seconds if waking up)%s...\n",
			apiURL, colorDim, colorReset)
	}

	client := api.NewClient(apiURL, verbose)
	start := time.Now()
	ok, err := client.Health()
	latency := time.Since(start)

	if err != nil {
		fmt.Printf("\n  %s✗ API server is unreachable%s at %s\n", colorRed, colorReset, apiURL)
		fmt.Printf("    %s\n", err)
		if !isLocal {
			fmt.Printf("\n  %sTip: The cloud server on Render spins down when idle.%s\n", colorYellow, colorReset)
			fmt.Printf("  %sIt may take 30-50 seconds to wake up. Try again shortly!%s\n", colorYellow, colorReset)
		}
		return
	}

	if ok {
		fmt.Printf("\n  %s✓ API server is online%s at %s\n", colorGreen, colorReset, apiURL)
		fmt.Printf("    Response time: %s%s%s\n", colorBold, latency.Round(time.Millisecond), colorReset)
	} else {
		fmt.Printf("\n  %s⚠ API server responded but may not be healthy%s at %s\n", colorYellow, colorReset, apiURL)
	}
}

func handleInstall(reader *bufio.Reader) {
	destPath := installer.InstalledBinaryPath()
	isInstalled := installer.IsInstalled()
	isInPath := installer.IsInPath()
	hasUpdate := isInstalled && installer.IsUpdateAvailable()

	if isInstalled && isInPath {
		if hasUpdate {
			fmt.Printf("\n  %s%s── Update Transfera in PATH ──%s\n\n", colorCyan, colorBold, colorReset)
			fmt.Printf("  %s🔄 New version / updated binary detected!%s\n", colorYellow, colorReset)
			fmt.Printf("    Installed: %s\n\n", destPath)
		} else {
			fmt.Printf("\n  %s%s── Reinstall / Update in PATH ──%s\n\n", colorCyan, colorBold, colorReset)
			fmt.Printf("  %s✓ Transfera is already installed and in PATH.%s\n", colorGreen, colorReset)
			fmt.Printf("    Installed: %s\n\n", destPath)
		}

		confirm := prompt(reader, fmt.Sprintf("  %s▶ Overwrite/reinstall binary? (Y/n):%s ", colorCyan, colorReset))
		confirm = strings.ToLower(strings.TrimSpace(confirm))
		if confirm != "" && confirm != "y" && confirm != "yes" {
			fmt.Printf("\n  %sNo changes made.%s\n", colorDim, colorReset)
			return
		}
		fmt.Println()
	} else {
		fmt.Printf("\n  %s%s── Install to PATH ──%s\n\n", colorCyan, colorBold, colorReset)
	}

	fmt.Printf("  Target directory: %s\n", installer.InstallDir())
	fmt.Printf("  Installing binary & verifying PATH...\n\n")

	msg, err := installer.Install()
	if err != nil {
		fmt.Printf("  %s✗ Installation failed: %s%s\n", colorRed, err, colorReset)
		return
	}

	fmt.Printf("  %s✓ %s%s\n", colorGreen, msg, colorReset)
}

func handleChangeServer(reader *bufio.Reader, apiURL *string) {
	fmt.Printf("\n  %s%s── Change Server URL ──%s\n\n", colorCyan, colorBold, colorReset)
	fmt.Printf("  Current server: %s%s%s\n\n", colorBold, *apiURL, colorReset)
	fmt.Printf("  %s1.%s Official cloud: https://transfera-api.onrender.com\n", colorGreen, colorReset)
	fmt.Printf("  %s2.%s Local server:   http://127.0.0.1:8080\n", colorGreen, colorReset)
	fmt.Printf("  %s3.%s Custom URL\n", colorGreen, colorReset)

	choice := prompt(reader, fmt.Sprintf("\n  %sSelect (1-3):%s ", colorWhite, colorReset))

	switch strings.TrimSpace(choice) {
	case "1":
		*apiURL = "https://transfera-api.onrender.com"
	case "2":
		*apiURL = "http://127.0.0.1:8080"
	case "3":
		custom := prompt(reader, fmt.Sprintf("  %sEnter server URL:%s ", colorWhite, colorReset))
		custom = strings.TrimSpace(custom)
		if custom != "" {
			*apiURL = custom
		}
	default:
		fmt.Printf("  %sNo changes made.%s\n", colorDim, colorReset)
		return
	}

	fmt.Printf("\n  %s✓ Server updated to: %s%s\n", colorGreen, *apiURL, colorReset)
}

func handleHelp() {
	fmt.Printf("\n  %s%s── CLI Quick Reference ──%s\n\n", colorCyan, colorBold, colorReset)
	fmt.Printf("  %sUpload a file:%s\n", colorBold, colorReset)
	fmt.Printf("    transfera upload photo.jpg\n")
	fmt.Printf("    transfera upload video.mp4 --max-downloads 5\n\n")
	fmt.Printf("  %sDownload a file:%s\n", colorBold, colorReset)
	fmt.Printf("    transfera download 52341\n")
	fmt.Printf("    transfera download 52341 -o ~/Pictures/\n\n")
	fmt.Printf("  %sCollaborative rooms:%s\n", colorBold, colorReset)
	fmt.Printf("    transfera room create --name Alice -i\n")
	fmt.Printf("    transfera room join 52341 --name Bob -i\n\n")
	fmt.Printf("  %sHealth check:%s\n", colorBold, colorReset)
	fmt.Printf("    transfera health\n\n")
	fmt.Printf("  %sGlobal flags:%s\n", colorBold, colorReset)
	fmt.Printf("    --api <url>    Set API server URL\n")
	fmt.Printf("    --verbose      Show HTTP details\n\n")
	fmt.Printf("  %sEnvironment variables:%s\n", colorBold, colorReset)
	fmt.Printf("    TRANSFERA_API_URL         Override default server\n")
	fmt.Printf("    NEXT_PUBLIC_API_BASE_URL  Alternative override\n")
}

// =========================================================================
// Utility functions
// =========================================================================

func prompt(reader *bufio.Reader, msg string) string {
	fmt.Print(msg)
	line, _ := reader.ReadString('\n')
	return strings.TrimSpace(strings.TrimRight(line, "\r\n"))
}

func promptContinue(reader *bufio.Reader) {
	fmt.Printf("  %sPress Enter to continue...%s", colorDim, colorReset)
	reader.ReadString('\n')
	clearScreen()
}

// cleanPath strips surrounding quotes, whitespace, expands tildes (~),
// and unescapes backslash-escaped spaces from terminal drag-and-drop on macOS/Linux.
func cleanPath(p string) string {
	p = strings.TrimSpace(p)
	p = strings.Trim(p, "\"'")

	// If file exists as-is, return it
	if _, err := os.Stat(p); err == nil {
		return p
	}

	// Expand tilde (~)
	if p == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	} else if strings.HasPrefix(p, "~/") || strings.HasPrefix(p, `~\`) {
		if home, err := os.UserHomeDir(); err == nil {
			p = filepath.Join(home, p[2:])
		}
	}

	// Handle terminal drag-and-drop on macOS/Linux escaping spaces with "\ "
	if strings.Contains(p, `\ `) {
		unescaped := strings.ReplaceAll(p, `\ `, " ")
		if _, err := os.Stat(unescaped); err == nil {
			return unescaped
		}
		p = unescaped
	}

	return strings.TrimSpace(p)
}
