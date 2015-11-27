require 'json'

class Blame
  attr_reader :file, :repo, :excluded

  def initialize(args)
    @authors = Hash.new(0)
    @author_scores = Hash.new(0.0)
    @line = 0
    @cur_ts = Time.now.to_i
    @ready = false
    args.each do |k,v|
      instance_variable_set("@#{k}", v) unless v.nil?
    end
    self.populate
  end

  def populate
    unless @ready
      Dir.chdir(@repo)
      author = nil
      File.readlines(@file).each do |li|
        if match = li.encode('utf-8', 'utf-8', :invalid => :replace).match(/\@owner\s+(.+)$/i)
          newauthor = match.captures
          if author.nil? 
            author = newauthor[0]
          else 
            author += ";" + newauthor[0]
          end
        end
      
      end
      if author
        @authors[author] = 1
        @author_scores[author] = 1
        @line +=1 
        return 
      end
      author = nil

      `git blame '#{@file}' --line-porcelain -w`
        .encode('utf-8', 'utf-8', :invalid => :replace).lines.each do |line|
        if /^author-mail <(.+)>$/ =~ line
          author = line[/^author-mail <(.+)>$/, 1]
          if @excluded.nil? or not @excluded.include? author.downcase
            @authors[author] += 1
          end
          @line += 1
        end
        if /^author-time (.+)$/ =~ line
          if @excluded.nil? or not @excluded.include? author.downcase
            ts = line[/^author-time (.+)$/, 1].to_i
            @author_scores[author] += 1.0/(1+(@cur_ts - ts)/60/60/24/90) # 90 days
          end
        end
      end
      @ready = true
    end
  end

  def top_authors(n)
    @author_scores.sort_by {|k,v| -v}
      .first(n)
      .map{|k,v| [k, @authors[k]]}
  end

  def output(n)
    unless @line == 0
      puts "#{@file},#{@line},#{top_authors(n).map{|a| a.join(",")}.join(",")}"
    end
  end
end
