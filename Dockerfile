FROM jekyll/jekyll:3.5

VOLUME $PWD:/srv/jekyll
VOLUME $PWD/vendor/bundle:/usr/local/bundle

CMD jekyll build
