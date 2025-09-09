# Minfer CLI

The Minfer command-line interface (CLI) lets you interact with MatrixInfer right from your terminal.

Below are the recommended ways to install Minfer on your platform.

- Windows: Winget or Scoop
- macOS and Linux: Homebrew
- All platforms: Download from GitHub Releases

## Installation

The `minfer` CLI can be installed through various package managers depending on your operating system, or directly downloaded from the GitHub release page.

### Windows

For Windows users, `minfer` can be installed using either Scoop or Winget.

#### Using Scoop

If you have Scoop installed, you can install `minfer` directly:

```bash
scoop install minfer
```

#### Using Winget

If you have Winget installed, you can install `minfer` using its full package ID:

```bash
winget install matrixinfer.minfer
```
*Note: While `winget install minfer` might sometimes work, `winget install matrixinfer.minfer` is the most explicit and reliable command to ensure you're installing the correct package if its official ID is `matrixinfer.minfer`.*

### Linux and macOS

For Linux and macOS users, `minfer` can be installed using Homebrew.

#### Using Homebrew

If you have Homebrew installed, you can install `minfer` by tapping the `matrixinfer-ai` tap:

```bash
brew tap matrixinfer-ai/matrixinfer
brew install minfer
```

### All Platforms (Direct Download)

You can also download the `minfer` CLI directly from the GitHub release page, which is suitable for all supported platforms.

1.  **Visit the GitHub Release Page:** Go to `https://github.com/matrixinfer-ai/matrixinfer/releases`
2.  **Download the Appropriate Release:** On the releases page, find the latest version and download the executable file that matches your operating system (e.g., `minfer-windows-amd64.exe` for Windows, `minfer-linux-amd64` for Linux, `minfer-darwin-amd64` for macOS).
3.  **Place in your PATH (Optional but Recommended):**
    *   **Windows:** Move the downloaded executable to a directory that is included in your system's `PATH` environment variable, or add the directory where you've placed it to your `PATH`.
    *   **Linux/macOS:** Move the downloaded executable to a directory like `/usr/local/bin` (or another directory in your `PATH`) and make it executable:
        ```bash
        sudo mv /path/to/downloaded/minfer /usr/local/bin/minfer
        sudo chmod +x /usr/local/bin/minfer
        ```

After installation, you should be able to run `minfer --help` in your terminal to verify the installation and see available commands.