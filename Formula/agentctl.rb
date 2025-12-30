# typed: false
# frozen_string_literal: true

# Homebrew formula for agentctl
# To use: brew install iheanyi/tap/agentctl
class Agentctl < Formula
  desc "Universal agent configuration manager for MCP servers"
  homepage "https://github.com/iheanyi/agentctl"
  license "MIT"
  head "https://github.com/iheanyi/agentctl.git", branch: "main"

  # Stable release (update version and checksums for releases)
  # url "https://github.com/iheanyi/agentctl/archive/refs/tags/v0.1.0.tar.gz"
  # sha256 "REPLACE_WITH_ACTUAL_SHA256"
  # version "0.1.0"

  depends_on "go" => :build

  def install
    ldflags = %W[
      -s -w
      -X github.com/iheanyi/agentctl/internal/cli.Version=#{version}
      -X github.com/iheanyi/agentctl/internal/cli.Commit=#{tap.user}
    ]

    system "go", "build", *std_go_args(ldflags:), "./cmd/agentctl"

    # Generate shell completions
    generate_completions_from_executable(bin/"agentctl", "completion")
  end

  def caveats
    <<~EOS
      To get started with agentctl:

        # Initialize configuration
        agentctl init

        # Install an MCP server
        agentctl install filesystem

        # Sync to your tools
        agentctl sync

      For more information, see: https://github.com/iheanyi/agentctl
    EOS
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/agentctl version")

    # Test config command works
    assert_match "Configuration", shell_output("#{bin}/agentctl config")
  end
end
