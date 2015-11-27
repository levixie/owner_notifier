git-ownership-anlysis
=====================

Print out top commiters per files

Usage: main.rb [options]
    -r, --repo repository            The location for the repository that you want to analysize
    -e, --exclude x,y,z              A list of committers that you want to exclude
    -h, --help                       Print usage.

Example: ruby main.rb -r ~/git/repo1 -e "kyle sun,anotherperson"
