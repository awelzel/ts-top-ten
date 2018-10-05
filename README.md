# ttt - tagesschau top ten - very rough go http app

## introduction

Getting a better grasp of things that are in the news.

This is crawls tagesschau.de for the top 10 news and provides a way to query
the most popular news per day based on a naive rating function.

This is all work in progress and my first shot at using golang - be warned.

## deployment

Currently supposed to run on Google App Engine.

### cron.yaml:

    cron:
      - description: "crawl tagesschau top 10"
        url: /ts
        schedule: every 5 minutes synchronized

### app.yaml:

    runtime: go
    env: flex
    env_variables:
      CLOUDSQL_CONNECTION_STRING: unix(/cloudsql/app:region:db)/tagesschau?time_zone=UTC&loc=UTC
      CLOUDSQL_USER: user
      CLOUDSQL_PASSWORD: password

    beta_settings:
      cloud_sql_instances: app:region:db
