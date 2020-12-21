module go.avito.ru/DO/moira

require (
	github.com/Knetic/govaluate v0.0.0-20170815164058-89a078c30383
	github.com/aclements/go-moremath v0.0.0-20190830160640-d16893ddf098 // indirect
	github.com/aristanetworks/goarista v0.0.0-20201022192228-4e6fdcf7f221
	github.com/armon/go-radix v0.0.0-20170727155443-1fca145dffbc // indirect
	github.com/carlosdp/twiliogo v0.0.0-20161027183705-b26045ebb9d1
	github.com/garyburd/redigo v1.6.0
	github.com/getsentry/sentry-go v0.7.0
	github.com/go-chi/chi v0.0.0-20170712121200-4c5a584b324b
	github.com/go-chi/render v1.0.0
	github.com/go-graphite/carbonapi v0.1.0
	github.com/golang/mock v1.4.3
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/gonum/blas v0.0.0-20181208220705-f22b278b28ac // indirect
	github.com/gonum/floats v0.0.0-20181209220543-c233463c7e82 // indirect
	github.com/gonum/internal v0.0.0-20181124074243-f884aa714029 // indirect
	github.com/gonum/lapack v0.0.0-20181123203213-e4cdc5a0bff9 // indirect
	github.com/gonum/matrix v0.0.0-20181209220409-c518dec07be9 // indirect
	github.com/gopherjs/gopherjs v0.0.0-20200217142428-fce0ec30dd00 // indirect
	github.com/gosexy/to v0.0.0-20141221203644-c20e083e3123
	github.com/gregdel/pushover v0.0.0-20161219170206-3c2e00dda05a
	github.com/lomik/zapwriter v0.0.0-20180906104450-2ec2b9a61680 // indirect
	github.com/mitchellh/hashstructure v0.0.0-20170609045927-2bca23e0e452 // indirect
	github.com/mitchellh/panicwrap v1.0.0
	github.com/mjibson/go-dsp v0.0.0-20180508042940-11479a337f12 // indirect
	github.com/op/go-logging v0.0.0-20160315200505-970db520ece7
	github.com/patrickmn/go-cache v2.1.0+incompatible
	github.com/pkg/errors v0.9.1
	github.com/rcrowley/go-metrics v0.0.0-20200313005456-10cdbea86bc0
	github.com/rs/cors v0.0.0-20170801073201-eabcc6af4bbe
	github.com/satori/go.uuid v1.2.0
	github.com/segmentio/fasthash v1.0.3
	github.com/slack-go/slack v0.7.2
	github.com/smartystreets/assertions v0.0.0-20180927180507-b2de0cb4f26d
	github.com/smartystreets/goconvey v1.6.4
	github.com/spf13/viper v1.6.3 // indirect
	github.com/stvp/tempredis v0.0.0-20181119212430-b82af8480203 // indirect
	github.com/tucnak/telebot v0.0.0-20170912115553-00cebf376d79
	go.uber.org/zap v1.15.0 // indirect
	golang.org/x/net v0.0.0-20200226121028-0de0cce0169b // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/alexcesaro/statsd.v2 v2.0.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
	gopkg.in/redsync.v1 v1.0.1
	gopkg.in/tomb.v2 v2.0.0-20161208151619-d5d1b5820637
	gopkg.in/yaml.v2 v2.2.8
)

// replace github.com/go-graphite/carbonapi => go.avito.ru/DO/carbonapi v0.0.0-20200421145808-de947e97f18f
replace github.com/go-graphite/carbonapi => github.com/el-yurchito/carbonapi v0.0.0-20201111142428-7f9c4c0535d1

go 1.13
