# m2pg

M2pg collects & serves metrics via an HTTP API. This service was designed to be an outlet of [l2met](https://github.com/ryandotsmith/l2met). Furthermore, m2pg is an open source alternative to services like [librato](https://metrics.librato.com/).

M2pg is designed to be a simple UNIX like utility. It's primary goal is to provide mechanisms that make storing and retrieving metrics simple and easy. In addition to developer experience, m2pg is designed to run on modern platforms like Heroku. It uses HTTP receivers which can be positioned behind load balancers. M2pg also makes special use of the PostgreSQL RDBMS. Specifically, it can safely use multiple database connections to store data in redundant systems.

## How It Works

M2pg is designed to accept input from [l2met](https://github.com/ryandotsmith/l2met).

![img](http://f.cl.ly/items/301Z0i3u0q0j0H301g3Z/arch.png)

M2pg uses a list of independent databases to redundantly store metrics. The algorithm is as follows:

* Parse metric from HTTP packet.
* Assign a UUID for the metric.
* For each database listed in DATABASE_URLS
* -> Start goroutine.
* -> Attempt insert into metrics table in database.
* Return on 1st successful insert.
* Log any failed insert.

M2pg reads from all databases listed in DATABASE_URLS. De-duplication is preformed on the collected metrics using the UUID. There is no preference on which metric is selected.

![img](http://f.cl.ly/items/0O0P0g3P3u3V0Q0p1q2R/arch.png)

### Known Issues

This method of HA is simple but expensive. It requires multiple RDBMS. You will want to make sure that each server is in disjoint availability zones. If the process corrupts or changes the data (however unlikely) in-between database inserts, subsequent queries may report inconsistent data. In practice this has never been an issue but it is theoretically possible.

## Deploy to Heroku

Download source and create a Heroku app.

```bash
$ git clone https://github.com/ryandotsmith/m2pg.git
$ heroku create myapp -b https://github.com/kr/heroku-buildpack-go.git
```

Setup a couple of dev databases & setup our DATABASE_URLS list. Notice the `|` separator.

```bash
$ heroku addons:add heroku-postgresql:dev
$ heroku addons:add heroku-postgresql:dev
$ heroku config:add DATABASE_URLS=postgres://...1|postgres://...2
```

Deploy source and start processes.

```bash
$ git push heroku master
$ heroku scale web=1 gc=1
```
