package db

import (
  "log"
  "database/sql"
  //"github.com/jmoiron/sqlx"
  _ "github.com/go-sql-driver/mysql"
  "alertstrap/config"
)

var (
  Conn    *sql.DB
  Conf    config.Config
)

func ConnectDb(cfg config.Config) (*sql.DB) {
  Conf = cfg
  db, err := sql.Open("mysql", cfg.Mysql.Conn_string)
  if err != nil {
    log.Printf("[error] %v", err)
    return nil
  }
  return db
}

func AddAlert(alrt map[string]interface{}) {
  //tx := Conn.MustBegin()
  //_, err := tx.NamedExec(`
  //  insert into `+Conf.Mysql.Alerts_table+`
  //    (
  //      mess_id, mgrp_id, host, severity, sv_level, ts_unix, text,
  //      instance, param, object, url
  //    )
  //  values
  //    (
  //      :mess_id, :mgrp_id, :host, :severity, :sv_level, :ts_unix, :text,
  //      :instance, :param, :object, :stand, :url
  //    )
  //`, alrt)
  //if err != nil {
  //  log.Printf("[error] %v", err)
  //  return
  //}
  //tx.Commit()
  return
}

func UpdAlert(alrt map[string]interface{}) {
  //tx := Conn.MustBegin()
  //_, err := tx.NamedExec(`
  //  update `+Conf.Mysql.Alerts_table+`
  //  set text=:text, duplicate=:duplicate, ts_max=now(), ts_unix=:ts_unix
  //`, alrt)
  //if err != nil {
  //  log.Printf("[error] %v", err)
  //  return
  //}
  //tx.Commit()
  return
}

func LoadAlerts() ([]map[string]interface{}) {
/*
  rows, err := Conn.Query("select * from "+Conf.Mysql.Alerts_view)
  if err != nil {
    log.Printf("[error] %v", err)
    return nil
  }

  var alts []map[string]interface{}
  for rows.Next() {
    var a map[string]interface{}
    err = rows.StructScan(&a)
    alts = append(alts, a)
  }

  return alts
*/
  var alts []map[string]interface{}
  return alts
}

func UpdateTask(mgrp_id string, task_key string) bool {
  key := GetTaskKey(mgrp_id)
  if key != "" {
    _, err := Conn.Exec("update "+Conf.Mysql.Tasks_table+" set task_key=? where mgrp_id=?", task_key, mgrp_id)
  	if err != nil {
      log.Printf("[error] %v", err)
  		return false
  	}
  } else {
    _, err := Conn.Exec("insert into "+Conf.Mysql.Tasks_table+" (mgrp_id, task_key) values (?, ?)", mgrp_id, task_key)
  	if err != nil {
      log.Printf("[error] %v", err)
  		return false
  	}
  }
  return true
}

func GetTaskKey(mgrp_id string) string {
/*
  var task_key string
	err := Conn.Get(&task_key, "select task_key from "+Conf.Mysql.Tasks_table+" where mgrp_id=?", mgrp_id)
  if err != nil {
    log.Printf("[error] %v", err)
    return ""
  }
  return task_key
*/
  return ""
}

func CreateSchema() {
/*
  Conn.MustExec(`
    create table if not exists `+Conf.Mysql.Alerts_table+` (
      mess_id varchar(50) not null,
      mgrp_id varchar(50) not null,
      host varchar(100) not null,
      real_host varchar(100),
      severity varchar(10) not null,
      sv_level int default 0,
      ts_min datetime default now(),
      ts_max datetime default now(),
      ts_unix bigint default 0,
      text text,
      duplicate int default 1,
      port_id int,
      appl_id varchar(10),
      instance varchar(50),
      mib varchar (250),
      param varchar (255),
      object varchar (350),
      short_oid varchar (200),
      full_oid varchar (1500),
      stand varchar (50),
      zone varchar (50),
      place varchar (50),
      url varchar(500),
      unique key IDX_mon_alerts_mess_id (mess_id),
      key IDX_mon_alerts_mgrp_id (mgrp_id),
      key IDX_mon_alerts_ts_max (ts_max),
      key IDX_mon_alerts_mgrp_id_ts_max (mgrp_id,ts_max)
    ) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
  `)
  Conn.MustExec(`
    create table if not exists `+Conf.Mysql.Tasks_table+` (
      mgrp_id varchar(50) not null,
      task_key varchar(10),
      date datetime default now(),
      unique key IDX_mon_alerts_mgrp_id (mgrp_id)
    ) engine InnoDB default charset=utf8mb4 collate=utf8mb4_unicode_ci
  `)
  Conn.MustExec(`
    create or replace view `+Conf.Mysql.Alerts_view+` as
    select
      mes.mess_id, mes.mgrp_id, mes.host, mes.severity, mes.sv_level,
      date_format(convert_tz(mes.ts_min, '+00:00', '-03:00'), '%Y-%m-%dT%H:%i:%sZ') as ts_min,
      date_format(convert_tz(mes.ts_max, '+00:00', '-03:00'), '%Y-%m-%dT%H:%i:%sZ') as ts_max,
      mes.ts_unix, mes.text, mes.duplicate,
      mes.port_id, mes.appl_id, mes.instance, mes.mib, mes.param, mes.object,
      mes.short_oid, mes.full_oid, mes.stand, mes.zone, mes.place, mes.url
    from `+Conf.Mysql.Alerts_table+` as mes
    inner join (
      select mess_id, mgrp_id, max(ts_max) as ts_max from `+Conf.Mysql.Alerts_table+`
      where ts_max > now() - interval 30 minute group by mess_id, mgrp_id
    ) as mes2 on mes.mgrp_id = mes2.mgrp_id and mes.ts_max = mes2.ts_max
    where mes.ts_max > now() - interval 30 minute
  `)
*/
}
