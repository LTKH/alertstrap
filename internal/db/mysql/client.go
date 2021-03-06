package mysql

import (
	"log"
	"fmt"
	"time"
	"strconv"
	"encoding/json"
	"database/sql"
	_ "github.com/go-sql-driver/mysql"
	"github.com/ltkh/alerttrap/internal/config"
	"github.com/ltkh/alerttrap/internal/cache"
)

type Client struct {
	client *sql.DB
    config *config.DB
}

func NewClient(conf *config.DB) (*Client, error) {
	conn, err := sql.Open("mysql", conf.Conn_string)
	if err != nil {
		return nil, err
	}
	return &Client{ client: conn, config: conf }, nil
}

func (db *Client) Healthy() error {
	stmt, err := db.client.Prepare(fmt.Sprintf(
		"select alert_id from %s a where a.ends_at > UNIX_TIMESTAMP() limit 1", 
		db.config.Alerts_table,
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	return nil
}

func (db *Client) LoadUser(login string) (cache.User, error) {
    var usr cache.User

    stmt, err := db.client.Prepare(fmt.Sprintf(
		"select login,name,password,token from %s where login = ?", 
		db.config.Users_table,
	))
	if err != nil {
		return usr, err
	}
	defer stmt.Close()

	err = stmt.QueryRow(login).Scan(&usr.Login, &usr.Name, &usr.Password, &usr.Token)
	if err != nil {
		return usr, err
	}

  	return usr, nil
}

func (db *Client) SaveUser(user cache.User) error {
	stmt, err := db.client.Prepare(fmt.Sprintf(
		"replace into %s (login,name,password,token) values (?,?,?,?)", 
		db.config.Users_table,
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	_, err = stmt.Exec(user.Login, user.Name, user.Password, user.Token)
	if err != nil {
		return err
	}

	return nil

}

func (db *Client) LoadUsers() ([]cache.User, error) {
	result := []cache.User{}

	rows, err := db.client.Query(fmt.Sprintf(
		"select login,password,token from %s", 
		db.config.Users_table,
	))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var usr cache.User
		usr.EndsAt = time.Now().UTC().Unix()
        err := rows.Scan(&usr.Login, &usr.Password, &usr.Token)
        if err != nil {
            return nil, err
		}
		result = append(result, usr) 
    }

  	return result, nil
}

func (db *Client) LoadAlerts() ([]cache.Alert, error) {
	result := []cache.Alert{}

	rows, err := db.client.Query(fmt.Sprintf(
		"select * from %s a where a.ends_at > UNIX_TIMESTAMP() - 600 and a.ends_at = (select max(ends_at) from %s where group_id = a.group_id)", 
		db.config.Alerts_table, 
		db.config.Alerts_table,
	))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.ColumnTypes()
	if err != nil {
		return nil, err
	}

	// Make a slice for the values
	values := make([]sql.RawBytes, len(columns))

	scanArgs := make([]interface{}, len(values))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() {
		var a cache.Alert

		if err := rows.Scan(scanArgs...); err != nil {
			continue
		}

		for i, value := range values {
			switch columns[i].Name() {
				case "alert_id":
					a.AlertId = string(value)
				case "group_id":
					a.GroupId = string(value)
				case "state":
					a.State = string(value)
				case "active_at":
					cl, err := strconv.Atoi(string(value))
					if err == nil {
						a.ActiveAt = int64(cl)
					}
				case "starts_at":
					cl, err := strconv.Atoi(string(value))
					if err == nil {
						a.StartsAt = int64(cl)
					}
				case "ends_at":
					cl, err := strconv.Atoi(string(value))
					if err == nil {
						a.EndsAt = int64(cl)
					}
				case "repeat":
					cl, err := strconv.Atoi(string(value))
					if err == nil {
						a.Repeat = int(cl)
					}
				case "change_st":
					cl, err := strconv.Atoi(string(value))
					if err == nil {
						a.ChangeSt = int(cl)
					}
				case "labels":
					if err := json.Unmarshal(value, &a.Labels); err != nil {
						log.Printf("[warning] %v (%s)", err, a.AlertId)
					}
				case "annotations":
					if err := json.Unmarshal(value, &a.Annotations); err != nil {
						log.Printf("[warning] %v (%s)", err, a.AlertId)
					}
				case "generator_url":
					a.GeneratorURL = string(value)
			}
		}

		result = append(result, a) 
	}

  	return result, nil
}

func (db *Client) SaveAlerts(alerts map[string]cache.Alert) error {

	stmt, err := db.client.Prepare(fmt.Sprintf(
		"replace into %s values (?,?,?,?,?,?,?,?,?,?,?)", 
		db.config.Alerts_table,
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, i := range alerts {

		labels, err := json.Marshal(i.Labels)
		if err != nil {
			return err
		}

		annotations, err := json.Marshal(i.Annotations)
		if err != nil {
			return err
		}

		_, err = stmt.Exec(
			i.AlertId,
			i.GroupId,
			i.State,
			i.ActiveAt,
			i.StartsAt,
			i.EndsAt,
			i.Repeat,
			i.ChangeSt,
			labels,
			annotations,
			i.GeneratorURL,
		)
		if err != nil {
			return err
		}

	}

	return nil

}

func (db *Client) AddAlert(alert cache.Alert) error {

	stmt, err := db.client.Prepare(fmt.Sprintf(
		"insert into %s values (?,?,?,?,?,?,?,?,?,?)", 
		db.config.Alerts_table,
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	labels, err := json.Marshal(alert.Labels)
	if err != nil {
		return err
	}

	annotations, err := json.Marshal(alert.Annotations)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(
		alert.AlertId,
		alert.GroupId,
		alert.State,
		alert.StartsAt,
		alert.EndsAt,
		alert.Repeat,
		alert.ChangeSt,
		labels,
		annotations,
		alert.GeneratorURL,
	)
	if err != nil {
		return err
	}

	return nil
}

func (db *Client) UpdAlert(alert cache.Alert) error {

	stmt, err := db.client.Prepare(fmt.Sprintf(
		"update %s set state=?,ends_at=?,repeat=?,change_st=?,annotations=?,generator_url=? where alert_id = ?", 
		db.config.Alerts_table,
	))
	if err != nil {
		return err
	}
	defer stmt.Close()

	annotations, err := json.Marshal(alert.Annotations)
	if err != nil {
		return err
	}

	_, err = stmt.Exec(
		alert.State,
		alert.EndsAt,
		alert.Repeat,
		alert.ChangeSt,
		annotations,
		alert.GeneratorURL,
		alert.AlertId,
	)
	if err != nil {
		return err
	}

	return nil
}

func (db *Client) DeleteOldAlerts() (int64, error) {

	stmt, err := db.client.Prepare(fmt.Sprintf(
		"delete from %s where ends_at < UNIX_TIMESTAMP() - 86400 * ?", 
		db.config.Alerts_table,
	))
	if err != nil {
		return 0, err
	}
	defer stmt.Close()

	res, err := stmt.Exec(db.config.History_days)
	if err != nil {
		return 0, err
	}

	cnt, err := res.RowsAffected()
	if err != nil {
		return 0, err
	}

	return cnt, nil

}