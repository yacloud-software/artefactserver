package db

/*
 This file was created by mkdb-client.
 The intention is not to modify thils file, but you may extend the struct DBArtefactID
 in a seperate file (so that you can regenerate this one from time to time)
*/

/*
 PRIMARY KEY: ID
*/

/*
 postgres:
 create sequence artefactid_seq;

Main Table:

 CREATE TABLE artefactid (id integer primary key default nextval('artefactid_seq'),domain text not null  ,name text not null  ,url text not null  );

Alter statements:
ALTER TABLE artefactid ADD COLUMN IF NOT EXISTS domain text not null default '';
ALTER TABLE artefactid ADD COLUMN IF NOT EXISTS name text not null default '';
ALTER TABLE artefactid ADD COLUMN IF NOT EXISTS url text not null default '';


Archive Table: (structs can be moved from main to archive using Archive() function)

 CREATE TABLE artefactid_archive (id integer unique not null,domain text not null,name text not null,url text not null);
*/

import (
	"context"
	gosql "database/sql"
	"fmt"
	savepb "golang.conradwood.net/apis/artefact"
	"golang.conradwood.net/go-easyops/sql"
	"os"
)

var (
	default_def_DBArtefactID *DBArtefactID
)

type DBArtefactID struct {
	DB                  *sql.DB
	SQLTablename        string
	SQLArchivetablename string
}

func DefaultDBArtefactID() *DBArtefactID {
	if default_def_DBArtefactID != nil {
		return default_def_DBArtefactID
	}
	psql, err := sql.Open()
	if err != nil {
		fmt.Printf("Failed to open database: %s\n", err)
		os.Exit(10)
	}
	res := NewDBArtefactID(psql)
	ctx := context.Background()
	err = res.CreateTable(ctx)
	if err != nil {
		fmt.Printf("Failed to create table: %s\n", err)
		os.Exit(10)
	}
	default_def_DBArtefactID = res
	return res
}
func NewDBArtefactID(db *sql.DB) *DBArtefactID {
	foo := DBArtefactID{DB: db}
	foo.SQLTablename = "artefactid"
	foo.SQLArchivetablename = "artefactid_archive"
	return &foo
}

// archive. It is NOT transactionally save.
func (a *DBArtefactID) Archive(ctx context.Context, id uint64) error {

	// load it
	p, err := a.ByID(ctx, id)
	if err != nil {
		return err
	}

	// now save it to archive:
	_, e := a.DB.ExecContext(ctx, "archive_DBArtefactID", "insert into "+a.SQLArchivetablename+" (id,domain, name, url) values ($1,$2, $3, $4) ", p.ID, p.Domain, p.Name, p.URL)
	if e != nil {
		return e
	}

	// now delete it.
	a.DeleteByID(ctx, id)
	return nil
}

// Save (and use database default ID generation)
func (a *DBArtefactID) Save(ctx context.Context, p *savepb.ArtefactID) (uint64, error) {
	qn := "DBArtefactID_Save"
	rows, e := a.DB.QueryContext(ctx, qn, "insert into "+a.SQLTablename+" (domain, name, url) values ($1, $2, $3) returning id", p.Domain, p.Name, p.URL)
	if e != nil {
		return 0, a.Error(ctx, qn, e)
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, a.Error(ctx, qn, fmt.Errorf("No rows after insert"))
	}
	var id uint64
	e = rows.Scan(&id)
	if e != nil {
		return 0, a.Error(ctx, qn, fmt.Errorf("failed to scan id after insert: %s", e))
	}
	p.ID = id
	return id, nil
}

// Save using the ID specified
func (a *DBArtefactID) SaveWithID(ctx context.Context, p *savepb.ArtefactID) error {
	qn := "insert_DBArtefactID"
	_, e := a.DB.ExecContext(ctx, qn, "insert into "+a.SQLTablename+" (id,domain, name, url) values ($1,$2, $3, $4) ", p.ID, p.Domain, p.Name, p.URL)
	return a.Error(ctx, qn, e)
}

func (a *DBArtefactID) Update(ctx context.Context, p *savepb.ArtefactID) error {
	qn := "DBArtefactID_Update"
	_, e := a.DB.ExecContext(ctx, qn, "update "+a.SQLTablename+" set domain=$1, name=$2, url=$3 where id = $4", p.Domain, p.Name, p.URL, p.ID)

	return a.Error(ctx, qn, e)
}

// delete by id field
func (a *DBArtefactID) DeleteByID(ctx context.Context, p uint64) error {
	qn := "deleteDBArtefactID_ByID"
	_, e := a.DB.ExecContext(ctx, qn, "delete from "+a.SQLTablename+" where id = $1", p)
	return a.Error(ctx, qn, e)
}

// get it by primary id
func (a *DBArtefactID) ByID(ctx context.Context, p uint64) (*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByID"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where id = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByID: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByID: error scanning (%s)", e))
	}
	if len(l) == 0 {
		return nil, a.Error(ctx, qn, fmt.Errorf("No ArtefactID with id %v", p))
	}
	if len(l) != 1 {
		return nil, a.Error(ctx, qn, fmt.Errorf("Multiple (%d) ArtefactID with id %v", len(l), p))
	}
	return l[0], nil
}

// get it by primary id (nil if no such ID row, but no error either)
func (a *DBArtefactID) TryByID(ctx context.Context, p uint64) (*savepb.ArtefactID, error) {
	qn := "DBArtefactID_TryByID"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where id = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("TryByID: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("TryByID: error scanning (%s)", e))
	}
	if len(l) == 0 {
		return nil, nil
	}
	if len(l) != 1 {
		return nil, a.Error(ctx, qn, fmt.Errorf("Multiple (%d) ArtefactID with id %v", len(l), p))
	}
	return l[0], nil
}

// get all rows
func (a *DBArtefactID) All(ctx context.Context) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_all"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" order by id")
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("All: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, fmt.Errorf("All: error scanning (%s)", e)
	}
	return l, nil
}

/**********************************************************************
* GetBy[FIELD] functions
**********************************************************************/

// get all "DBArtefactID" rows with matching Domain
func (a *DBArtefactID) ByDomain(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByDomain"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where domain = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByDomain: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByDomain: error scanning (%s)", e))
	}
	return l, nil
}

// the 'like' lookup
func (a *DBArtefactID) ByLikeDomain(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByLikeDomain"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where domain ilike $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByDomain: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByDomain: error scanning (%s)", e))
	}
	return l, nil
}

// get all "DBArtefactID" rows with matching Name
func (a *DBArtefactID) ByName(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByName"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where name = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByName: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByName: error scanning (%s)", e))
	}
	return l, nil
}

// the 'like' lookup
func (a *DBArtefactID) ByLikeName(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByLikeName"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where name ilike $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByName: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByName: error scanning (%s)", e))
	}
	return l, nil
}

// get all "DBArtefactID" rows with matching URL
func (a *DBArtefactID) ByURL(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByURL"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where url = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByURL: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByURL: error scanning (%s)", e))
	}
	return l, nil
}

// the 'like' lookup
func (a *DBArtefactID) ByLikeURL(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByLikeURL"
	rows, e := a.DB.QueryContext(ctx, qn, "select id,domain, name, url from "+a.SQLTablename+" where url ilike $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByURL: error querying (%s)", e))
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, a.Error(ctx, qn, fmt.Errorf("ByURL: error scanning (%s)", e))
	}
	return l, nil
}

/**********************************************************************
* Helper to convert from an SQL Query
**********************************************************************/

// from a query snippet (the part after WHERE)
func (a *DBArtefactID) FromQuery(ctx context.Context, query_where string, args ...interface{}) ([]*savepb.ArtefactID, error) {
	rows, err := a.DB.QueryContext(ctx, "custom_query_"+a.Tablename(), "select "+a.SelectCols()+" from "+a.Tablename()+" where "+query_where, args...)
	if err != nil {
		return nil, err
	}
	return a.FromRows(ctx, rows)
}

/**********************************************************************
* Helper to convert from an SQL Row to struct
**********************************************************************/
func (a *DBArtefactID) Tablename() string {
	return a.SQLTablename
}

func (a *DBArtefactID) SelectCols() string {
	return "id,domain, name, url"
}
func (a *DBArtefactID) SelectColsQualified() string {
	return "" + a.SQLTablename + ".id," + a.SQLTablename + ".domain, " + a.SQLTablename + ".name, " + a.SQLTablename + ".url"
}

func (a *DBArtefactID) FromRows(ctx context.Context, rows *gosql.Rows) ([]*savepb.ArtefactID, error) {
	var res []*savepb.ArtefactID
	for rows.Next() {
		foo := savepb.ArtefactID{}
		err := rows.Scan(&foo.ID, &foo.Domain, &foo.Name, &foo.URL)
		if err != nil {
			return nil, a.Error(ctx, "fromrow-scan", err)
		}
		res = append(res, &foo)
	}
	return res, nil
}

/**********************************************************************
* Helper to create table and columns
**********************************************************************/
func (a *DBArtefactID) CreateTable(ctx context.Context) error {
	csql := []string{
		`create sequence if not exists ` + a.SQLTablename + `_seq;`,
		`CREATE TABLE if not exists ` + a.SQLTablename + ` (id integer primary key default nextval('` + a.SQLTablename + `_seq'),domain text not null ,name text not null ,url text not null );`,
		`CREATE TABLE if not exists ` + a.SQLTablename + `_archive (id integer primary key default nextval('` + a.SQLTablename + `_seq'),domain text not null ,name text not null ,url text not null );`,
		`ALTER TABLE artefactid ADD COLUMN IF NOT EXISTS domain text not null default '';`,
		`ALTER TABLE artefactid ADD COLUMN IF NOT EXISTS name text not null default '';`,
		`ALTER TABLE artefactid ADD COLUMN IF NOT EXISTS url text not null default '';`,

		`ALTER TABLE artefactid_archive ADD COLUMN IF NOT EXISTS domain text not null default '';`,
		`ALTER TABLE artefactid_archive ADD COLUMN IF NOT EXISTS name text not null default '';`,
		`ALTER TABLE artefactid_archive ADD COLUMN IF NOT EXISTS url text not null default '';`,
	}
	for i, c := range csql {
		_, e := a.DB.ExecContext(ctx, fmt.Sprintf("create_"+a.SQLTablename+"_%d", i), c)
		if e != nil {
			return e
		}
	}

	// these are optional, expected to fail
	csql = []string{
		// Indices:

		// Foreign keys:

	}
	for i, c := range csql {
		a.DB.ExecContextQuiet(ctx, fmt.Sprintf("create_"+a.SQLTablename+"_%d", i), c)
	}
	return nil
}

/**********************************************************************
* Helper to meaningful errors
**********************************************************************/
func (a *DBArtefactID) Error(ctx context.Context, q string, e error) error {
	if e == nil {
		return nil
	}
	return fmt.Errorf("[table="+a.SQLTablename+", query=%s] Error: %s", q, e)
}



