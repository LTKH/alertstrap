package main

import (
  "net/http"
  "time"
  "log"
  "os"
  "os/signal"
  "syscall"
  "runtime"
  "flag"
  "github.com/ltkh/alertstrap/internal/db"
  "github.com/ltkh/alertstrap/internal/api/v1"
  "github.com/ltkh/alertstrap/internal/config"
  "github.com/ltkh/alertstrap/internal/monitor"
)

func main() {

	//limits the number of operating system threads
	runtime.GOMAXPROCS(runtime.NumCPU())

	//command-line flag parsing
	cfFile := flag.String("config", "", "config file")
	//dBase  := flag.String("dbase", "", "sql file")
	flag.Parse()

	//loading configuration file
	cfg, err := config.New(*cfFile)
	if err != nil {
		log.Fatalf("[error] %v", err)
	}

	//program completion signal processing
	c := make(chan os.Signal, 2)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)
	go func() {
		<- c
		//connection to data base
		client, err := db.NewClient(&cfg.DB); 
		if err != nil {
			log.Fatalf("[error] %v", err)
		}
		//saving cache items
		if items := v1.CacheAlerts.Items(); len(items) != 0 {
			client.SaveAlerts(items)
		}
		log.Print("[info] alertstrap stopped")
		os.Exit(0)
	}()

	//creating api
	apiV1, _ := v1.New(&cfg)

	//enabled listen port
	http.HandleFunc("/api/v1/login", apiV1.ApiLogin)
	http.HandleFunc("/api/v1/menu", apiV1.ApiMenu)
	http.HandleFunc("/api/v1/alerts", apiV1.ApiAlerts)

	go func(cfg *config.Server){
		if cfg.Cert_file != "" && cfg.Cert_key != "" {
			if err := http.ListenAndServeTLS(cfg.Listen, cfg.Cert_file, cfg.Cert_key, nil); err != nil {
				log.Fatalf("[error] %v", err)
			}
		} else {
			if err := http.ListenAndServe(cfg.Listen, nil); err != nil {
				log.Fatalf("[error] %v", err)
			}
		}
		log.Printf("[info] listen port enabled - %s", cfg.Listen)
	}(&cfg.Server)

	//opening monitoring port
	monitor.Start(cfg.Monit.Listen)

	log.Print("[info] alertstrap started -_^")

	//delete old alerts
	go func(cfg *config.DB){
		for {
			//connection to data base
			client, err := db.NewClient(cfg); 
			defer client.Close()
			if err != nil {
				log.Printf("[error] %v", err)
			} else {
				//cleaning old alerts
				if err := client.DeleteOldAlerts(); err != nil {
					log.Printf("[error] %v", err)
				} else {
					log.Print("[info] old alerts removed")
				}
			}
			time.Sleep(24 * time.Hour)
		}
	}(&cfg.DB)

	//daemon mode
	for {

		//mark alerts as resolved
		if keys := v1.CacheAlerts.ResolvedItems(); len(keys) != 0 {
            log.Printf("[info] mark alerts as resolved (%d)", len(keys))
		}

		//connection to data base
		client, err := db.NewClient(&cfg.DB); 
		defer client.Close()
		if err != nil {
			log.Printf("[error] %v", err)
		} else {
			//cleaning cache items
			if items := v1.CacheAlerts.ExpiredItems(); len(items) != 0 {
				client.SaveAlerts(items)
				v1.CacheAlerts.ClearItems(items)
			}
	    }

		time.Sleep(10 * time.Second)

	}
}
