require_relative "repo.rb"
require 'optparse'

options = {}
OptionParser.new do |opts|
  opts.banner = "Usage: main.rb [options]"

  opts.on("-r", "--repo repository",
            "The location for the repository that you want to analysize") do |repo|
    options[:repo] = repo
  end

  opts.on("-e", "--exclude x,y,z", Array,
            "A list of committers that you want to exclude") do |list|
    options[:excluded] = list
  end

  opts.on_tail("-h", "--help", "Print usage.") do
    puts opts
    exit
  end
end.parse!

Repo.new({repo: options[:repo], excluded: options[:excluded]}).blame
