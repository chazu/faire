# Homebrew formula for faire
# To install:
#   brew install chazu/faire/faire
# Or from a local file:
#   brew install ./Formula/faire.rb

class Faire < Formula
  homepage "https://github.com/chazu/faire"
  url "https://github.com/chazu/faire/archive/refs/tags/vVERSION.tar.gz"
  sha256 "SHA256_PLACEHOLDER"
  license "MIT"

  depends_on "go" => :build

  def install
    system "go", "build", *std_go_args(ldflags: "-X main.Version=vVERSION -X main.Commit=#{tap.user}-brew"), "./cmd/gitsavvy"
  end

  test do
    system bin/"faire", "--version"
  end
end
