class Macscope < Formula
  desc "macOS introspection and triage toolkit"
  homepage "https://github.com/jdefrancesco/macscope"
  url "__MACSCOPE_URL__"
  version "__MACSCOPE_VERSION__"
  sha256 "__MACSCOPE_SHA256__"

  depends_on :macos

  def install
    bin.install "macscope"
    bash_completion.install "completions/macscope.bash" => "macscope"
    zsh_completion.install "completions/_macscope" => "_macscope"
    fish_completion.install "completions/macscope.fish"
    man1.install "man/man1/macscope.1"
  end

  test do
    assert_match version.to_s, shell_output("#{bin}/macscope version")
    assert_match "macscope <command> [flags]", shell_output("#{bin}/macscope help")
  end
end
