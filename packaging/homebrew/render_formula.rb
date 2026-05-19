#!/usr/bin/env ruby
# frozen_string_literal: true

require "fileutils"

template_path = ENV.fetch("HOMEBREW_TEMPLATE", File.expand_path("macscope.rb", __dir__))
output_path = ENV.fetch("HOMEBREW_FORMULA")

values = {
  "VERSION" => ENV.fetch("MACSCOPE_VERSION"),
  "URL" => ENV.fetch("MACSCOPE_URL"),
  "SHA256" => ENV.fetch("MACSCOPE_SHA256")
}

if values["VERSION"].empty?
  abort "MACSCOPE_VERSION is required"
end

unless values["URL"].match?(%r{\Ahttps?://\S+\z})
  abort "MACSCOPE_URL must be an http(s) URL"
end

unless values["SHA256"].match?(/\A[0-9a-f]{64}\z/i)
  abort "MACSCOPE_SHA256 must be a 64-character hex SHA-256"
end

content = File.read(template_path)
values.each do |name, value|
  content = content.gsub("__MACSCOPE_#{name}__", value)
end

remaining = content.scan(/__MACSCOPE_[A-Z0-9_]+__/).uniq
unless remaining.empty?
  abort "unrendered Homebrew template placeholders: #{remaining.join(", ")}"
end

FileUtils.mkdir_p(File.dirname(output_path))
File.write(output_path, content)
