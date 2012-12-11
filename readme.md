# m2pg

M2pg collects & serves metrics via an HTTP API. This service was designed to be an outlet of [l2met](https://github.com/ryandotsmith/l2met). Furthermore, m2pg is an open source alternative to services like [librato](https://metrics.librato.com/).

M2pg is designed to be a simple UNIX like utility. It's primary goal is to provide mechanisms that make storing and retrieving metrics simple and easy. In addition to developer experience, m2pg is designed to run on modern platforms like Heroku. It uses HTTP receivers which can be positioned behind load balancers. M2pg also makes special use of the PostgreSQL RDBMS. Specifically, it can safely use multiple database connections to store data in redundant systems.

## API

### Read

**GET /metrics ?name&from&to&resolution**

#### name

The name parameter helps m2pg find metrics. It behaves like a regular expression so queries like the following are valid and quite handy:

```
?name=my-app-(prod|staging)
```

#### from

Return no events less than this parameter. Format should be [rfc3339](http://www.ietf.org/rfc/rfc3339.txt) compatible.

#### to

Return no events greater than this parameter. Format should be [rfc3339](http://www.ietf.org/rfc/rfc3339.txt) compatible.

#### resolution

The metrics returned will be grouped by their resolution. Valid formats include:

* second, s
* minute, m
* hour, h
* day, d
* week, w
* month, m
* year, y

#### Example

```bash
$ curl -i -X GET \
  "https://m2pg.herokuapp.com/metrics?name=foo.web&from=1355202720&to=1355202759&resolution=minute"

HTTP/1.1 200 OK
Content-Type: application/json; charset=utf-8
Date: Tue, 11 Dec 2012 05:23:13 GMT
Transfer-Encoding: chunked

[
	{
		"name":    "foo.web.requests",
		"count":   [{"bucket": 1355202720, "value": 42}],
		"mean":    [{"bucket": 1355202720, "value": 3.14}],
		"median":  [{"bucket": 1355202720, "value": 3.14}],
		"min":     [{"bucket": 1355202720, "value": 3.14}],
		"max":     [{"bucket": 1355202720, "value": 3.14}],
		"perc95":  [{"bucket": 1355202720, "value": 3.14}],
		"perc99":  [{"bucket": 1355202720, "value": 3.14}],
		"last":    [{"bucket": 1355202720, "value": 3.14}],
	},
	{
		"name":    "foo.web.special-requests",
		"count":   [{"bucket": 1355202720, "value": 42}],
		"mean":    [{"bucket": 1355202720, "value": 3.14}],
		"median":  [{"bucket": 1355202720, "value": 3.14}],
		"min":     [{"bucket": 1355202720, "value": 3.14}],
		"max":     [{"bucket": 1355202720, "value": 3.14}],
		"perc95":  [{"bucket": 1355202720, "value": 3.14}],
		"perc99":  [{"bucket": 1355202720, "value": 3.14}],
		"last":    [{"bucket": 1355202720, "value": 3.14}],
	}
]
```

### Write

**POST /metrics**

```bash
$ curl -i -X POST https://m2pg.herokuapp.com/metrics \
  -d '{"bucket": 1355202720,
        "name": "foo.web.requests",
        "count": 100,
        "mean": 50,
        "median": 50,
        "min": 0,
        "max": 100,
        "perc95": 95,
        "perc99": 99,
        "last": 95}'

HTTP/1.1 201 Created
Content-Type: application/json; charset=utf-8
Date: Tue, 11 Dec 2012 05:19:24 GMT
Transfer-Encoding: chunked

"e5176747-cb3f-aaeb-7872-e00c353326c8"
```

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
