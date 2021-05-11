# What is this?

Dockerfiles and docker-compose.yml placed in this dir are used for dev purposes only.

# howto

1. Place your [moira-web2.0](https://github.com/avito-tech/moira-web) repo, so it is located at web2.0 folder of this project (you can use sub-repo for it) OR redact build context path for `web` service.
2. Make sure your local configs are alright. They are located at pkg folder.
3. Change to `docker` dir.
4. ```docker-compose build --parallel``` (optional)
5. ```docker-compose up```
