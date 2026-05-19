#!/usr/bin/env ruby
# frozen_string_literal: true

require "shellwords"

formula_path, expected_version, expected_url, expected_sha256 = ARGV

unless formula_path && expected_version && expected_url && expected_sha256
  abort "usage: verify_formula.rb <formula> <version> <url> <sha256>"
end

content = File.read(formula_path)

unless expected_sha256.match?(/\A[0-9a-f]{64}\z/i)
  abort "expected SHA-256 must be a 64-character hex string"
end

expected_lines = [
  "class Macscope < Formula",
  "url \"#{expected_url}\"",
  "version \"#{expected_version}\"",
  "sha256 \"#{expected_sha256}\"",
  "depends_on :macos",
  "bin.install \"macscope\"",
  "bash_completion.install \"completions/macscope.bash\" => \"macscope\"",
  "zsh_completion.install \"completions/_macscope\" => \"_macscope\"",
  "fish_completion.install \"completions/macscope.fish\"",
  "man1.install \"man/man1/macscope.1\"",
  "assert_match version.to_s, shell_output(\"\#{bin}/macscope version\")",
  "assert_match \"macscope <command> [flags]\", shell_output(\"\#{bin}/macscope help\")"
]

missing = expected_lines.reject { |line| content.include?(line) }
unless missing.empty?
  abort "formula missing expected content:\n#{missing.join("\n")}"
end

remaining = content.scan(/__MACSCOPE_[A-Z0-9_]+__/).uniq
unless remaining.empty?
  abort "formula still contains unrendered placeholders: #{remaining.join(", ")}"
end

syntax_output = `ruby -c #{Shellwords.escape(formula_path)} 2>&1`
unless $?.success?
  abort "formula Ruby syntax check failed:\n#{syntax_output}"
end

puts "Verified #{formula_path}"
