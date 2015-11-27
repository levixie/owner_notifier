require_relative 'blame.rb'

class Repo
  attr_reader :repo, :excluded

  def initialize(args)
    args.each do |k,v|
      instance_variable_set("@#{k}", v) unless v.nil?
    end
    @files = self.ls_files
    unless @excluded.nil?
      @excluded.map!{|x| x.downcase}
    end
  end

  def ls_files
    Dir.chdir(@repo)
    @files = `git ls-files`.split("\n")
      .find_all{|e|
        e.end_with?(".php", ".py", ".js", ".java", ".less") and
        not e.include?("ext/") and
        not e.include?("dw/") and
        not e.start_with?(".")
      }
  end

  def blame
    puts "file,line count,committer 1,line count 1,committer 2,line count 2,committer 3,line count 3"
    @files.each do |file|
      f = Blame.new({repo: @repo, file: file, excluded: excluded})
      f.output(3)
    end
  end

end
