package db

/*
 This file was created by mkdb-client.
 The intention is not to modify this file, but you may extend the struct DBArtefactID
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
	"golang.conradwood.net/go-easyops/errors"
	"golang.conradwood.net/go-easyops/sql"
	"os"
	"sync"
)

var (
	default_def_DBArtefactID *DBArtefactID
)

type DBArtefactID struct {
	DB                   *sql.DB
	SQLTablename         string
	SQLArchivetablename  string
	customColumnHandlers []CustomColumnHandler
	lock                 sync.Mutex
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

func (a *DBArtefactID) GetCustomColumnHandlers() []CustomColumnHandler {
	return a.customColumnHandlers
}
func (a *DBArtefactID) AddCustomColumnHandler(w CustomColumnHandler) {
	a.lock.Lock()
	a.customColumnHandlers = append(a.customColumnHandlers, w)
	a.lock.Unlock()
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

// return a map with columnname -> value_from_proto
func (a *DBArtefactID) buildSaveMap(ctx context.Context, p *savepb.ArtefactID) (map[string]interface{}, error) {
	extra, err := extraFieldsToStore(ctx, a, p)
	if err != nil {
		return nil, err
	}
	res := make(map[string]interface{})
	res["id"] = a.get_col_from_proto(p, "id")
	res["domain"] = a.get_col_from_proto(p, "domain")
	res["name"] = a.get_col_from_proto(p, "name")
	res["url"] = a.get_col_from_proto(p, "url")
	if extra != nil {
		for k, v := range extra {
			res[k] = v
		}
	}
	return res, nil
}

func (a *DBArtefactID) Save(ctx context.Context, p *savepb.ArtefactID) (uint64, error) {
	qn := "save_DBArtefactID"
	smap, err := a.buildSaveMap(ctx, p)
	if err != nil {
		return 0, err
	}
	delete(smap, "id") // save without id
	return a.saveMap(ctx, qn, smap, p)
}

// Save using the ID specified
func (a *DBArtefactID) SaveWithID(ctx context.Context, p *savepb.ArtefactID) error {
	qn := "insert_DBArtefactID"
	smap, err := a.buildSaveMap(ctx, p)
	if err != nil {
		return err
	}
	_, err = a.saveMap(ctx, qn, smap, p)
	return err
}

// use a hashmap of columnname->values to store to database (see buildSaveMap())
func (a *DBArtefactID) saveMap(ctx context.Context, queryname string, smap map[string]interface{}, p *savepb.ArtefactID) (uint64, error) {
	// Save (and use database default ID generation)

	var rows *gosql.Rows
	var e error

	q_cols := ""
	q_valnames := ""
	q_vals := make([]interface{}, 0)
	deli := ""
	i := 0
	// build the 2 parts of the query (column names and value names) as well as the values themselves
	for colname, val := range smap {
		q_cols = q_cols + deli + colname
		i++
		q_valnames = q_valnames + deli + fmt.Sprintf("$%d", i)
		q_vals = append(q_vals, val)
		deli = ","
	}
	rows, e = a.DB.QueryContext(ctx, queryname, "insert into "+a.SQLTablename+" ("+q_cols+") values ("+q_valnames+") returning id", q_vals...)
	if e != nil {
		return 0, a.Error(ctx, queryname, e)
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, a.Error(ctx, queryname, errors.Errorf("No rows after insert"))
	}
	var id uint64
	e = rows.Scan(&id)
	if e != nil {
		return 0, a.Error(ctx, queryname, errors.Errorf("failed to scan id after insert: %s", e))
	}
	p.ID = id
	return id, nil
}

func (a *DBArtefactID) Update(ctx context.Context, p *savepb.ArtefactID) error {
	qn := "DBArtefactID_Update"
	_, e := a.DB.ExecContext(ctx, qn, "update "+a.SQLTablename+" set domain=$1, name=$2, url=$3 where id = $4", a.get_Domain(p), a.get_Name(p), a.get_URL(p), p.ID)

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
	l, e := a.fromQuery(ctx, qn, "id = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByID: error scanning (%s)", e))
	}
	if len(l) == 0 {
		return nil, a.Error(ctx, qn, errors.Errorf("No ArtefactID with id %v", p))
	}
	if len(l) != 1 {
		return nil, a.Error(ctx, qn, errors.Errorf("Multiple (%d) ArtefactID with id %v", len(l), p))
	}
	return l[0], nil
}

// get it by primary id (nil if no such ID row, but no error either)
func (a *DBArtefactID) TryByID(ctx context.Context, p uint64) (*savepb.ArtefactID, error) {
	qn := "DBArtefactID_TryByID"
	l, e := a.fromQuery(ctx, qn, "id = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("TryByID: error scanning (%s)", e))
	}
	if len(l) == 0 {
		return nil, nil
	}
	if len(l) != 1 {
		return nil, a.Error(ctx, qn, errors.Errorf("Multiple (%d) ArtefactID with id %v", len(l), p))
	}
	return l[0], nil
}

// get it by multiple primary ids
func (a *DBArtefactID) ByIDs(ctx context.Context, p []uint64) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByIDs"
	l, e := a.fromQuery(ctx, qn, "id in $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("TryByID: error scanning (%s)", e))
	}
	return l, nil
}

// get all rows
func (a *DBArtefactID) All(ctx context.Context) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_all"
	l, e := a.fromQuery(ctx, qn, "true")
	if e != nil {
		return nil, errors.Errorf("All: error scanning (%s)", e)
	}
	return l, nil
}

/**********************************************************************
* GetBy[FIELD] functions
**********************************************************************/

// get all "DBArtefactID" rows with matching Domain
func (a *DBArtefactID) ByDomain(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByDomain"
	l, e := a.fromQuery(ctx, qn, "domain = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByDomain: error scanning (%s)", e))
	}
	return l, nil
}

// get all "DBArtefactID" rows with multiple matching Domain
func (a *DBArtefactID) ByMultiDomain(ctx context.Context, p []string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByDomain"
	l, e := a.fromQuery(ctx, qn, "domain in $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByDomain: error scanning (%s)", e))
	}
	return l, nil
}

// the 'like' lookup
func (a *DBArtefactID) ByLikeDomain(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByLikeDomain"
	l, e := a.fromQuery(ctx, qn, "domain ilike $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByDomain: error scanning (%s)", e))
	}
	return l, nil
}

// get all "DBArtefactID" rows with matching Name
func (a *DBArtefactID) ByName(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByName"
	l, e := a.fromQuery(ctx, qn, "name = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByName: error scanning (%s)", e))
	}
	return l, nil
}

// get all "DBArtefactID" rows with multiple matching Name
func (a *DBArtefactID) ByMultiName(ctx context.Context, p []string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByName"
	l, e := a.fromQuery(ctx, qn, "name in $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByName: error scanning (%s)", e))
	}
	return l, nil
}

// the 'like' lookup
func (a *DBArtefactID) ByLikeName(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByLikeName"
	l, e := a.fromQuery(ctx, qn, "name ilike $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByName: error scanning (%s)", e))
	}
	return l, nil
}

// get all "DBArtefactID" rows with matching URL
func (a *DBArtefactID) ByURL(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByURL"
	l, e := a.fromQuery(ctx, qn, "url = $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByURL: error scanning (%s)", e))
	}
	return l, nil
}

// get all "DBArtefactID" rows with multiple matching URL
func (a *DBArtefactID) ByMultiURL(ctx context.Context, p []string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByURL"
	l, e := a.fromQuery(ctx, qn, "url in $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByURL: error scanning (%s)", e))
	}
	return l, nil
}

// the 'like' lookup
func (a *DBArtefactID) ByLikeURL(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	qn := "DBArtefactID_ByLikeURL"
	l, e := a.fromQuery(ctx, qn, "url ilike $1", p)
	if e != nil {
		return nil, a.Error(ctx, qn, errors.Errorf("ByURL: error scanning (%s)", e))
	}
	return l, nil
}

/**********************************************************************
* The field getters
**********************************************************************/

// getter for field "ID" (ID) [uint64]
func (a *DBArtefactID) get_ID(p *savepb.ArtefactID) uint64 {
	return uint64(p.ID)
}

// getter for field "Domain" (Domain) [string]
func (a *DBArtefactID) get_Domain(p *savepb.ArtefactID) string {
	return string(p.Domain)
}

// getter for field "Name" (Name) [string]
func (a *DBArtefactID) get_Name(p *savepb.ArtefactID) string {
	return string(p.Name)
}

// getter for field "URL" (URL) [string]
func (a *DBArtefactID) get_URL(p *savepb.ArtefactID) string {
	return string(p.URL)
}

/**********************************************************************
* Helper to convert from an SQL Query
**********************************************************************/

// from a query snippet (the part after WHERE)
func (a *DBArtefactID) ByDBQuery(ctx context.Context, query *Query) ([]*savepb.ArtefactID, error) {
	extra_fields, err := extraFieldsToQuery(ctx, a)
	if err != nil {
		return nil, err
	}
	i := 0
	for col_name, value := range extra_fields {
		i++
		efname := fmt.Sprintf("EXTRA_FIELD_%d", i)
		query.Add(col_name+" = "+efname, QP{efname: value})
	}

	gw, paras := query.ToPostgres()
	queryname := "custom_dbquery"
	rows, err := a.DB.QueryContext(ctx, queryname, "select "+a.SelectCols()+" from "+a.Tablename()+" where "+gw, paras...)
	if err != nil {
		return nil, err
	}
	res, err := a.FromRows(ctx, rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return res, nil

}

func (a *DBArtefactID) FromQuery(ctx context.Context, query_where string, args ...interface{}) ([]*savepb.ArtefactID, error) {
	return a.fromQuery(ctx, "custom_query_"+a.Tablename(), query_where, args...)
}

// from a query snippet (the part after WHERE)
func (a *DBArtefactID) fromQuery(ctx context.Context, queryname string, query_where string, args ...interface{}) ([]*savepb.ArtefactID, error) {
	extra_fields, err := extraFieldsToQuery(ctx, a)
	if err != nil {
		return nil, err
	}
	eq := ""
	if extra_fields != nil && len(extra_fields) > 0 {
		eq = " AND ("
		// build the extraquery "eq"
		i := len(args)
		deli := ""
		for col_name, value := range extra_fields {
			i++
			eq = eq + deli + col_name + fmt.Sprintf(" = $%d", i)
			deli = " AND "
			args = append(args, value)
		}
		eq = eq + ")"
	}
	rows, err := a.DB.QueryContext(ctx, queryname, "select "+a.SelectCols()+" from "+a.Tablename()+" where ( "+query_where+") "+eq, args...)
	if err != nil {
		return nil, err
	}
	res, err := a.FromRows(ctx, rows)
	rows.Close()
	if err != nil {
		return nil, err
	}
	return res, nil
}

/**********************************************************************
* Helper to convert from an SQL Row to struct
**********************************************************************/
func (a *DBArtefactID) get_col_from_proto(p *savepb.ArtefactID, colname string) interface{} {
	if colname == "id" {
		return a.get_ID(p)
	} else if colname == "domain" {
		return a.get_Domain(p)
	} else if colname == "name" {
		return a.get_Name(p)
	} else if colname == "url" {
		return a.get_URL(p)
	}
	panic(fmt.Sprintf("in table \"%s\", column \"%s\" cannot be resolved to proto field name", a.Tablename(), colname))
}

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
		// SCANNER:
		foo := &savepb.ArtefactID{}
		// create the non-nullable pointers
		// create variables for scan results
		scanTarget_0 := &foo.ID
		scanTarget_1 := &foo.Domain
		scanTarget_2 := &foo.Name
		scanTarget_3 := &foo.URL
		err := rows.Scan(scanTarget_0, scanTarget_1, scanTarget_2, scanTarget_3)
		// END SCANNER

		if err != nil {
			return nil, a.Error(ctx, "fromrow-scan", err)
		}
		res = append(res, foo)
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
		`ALTER TABLE ` + a.SQLTablename + ` ADD COLUMN IF NOT EXISTS domain text not null default '';`,
		`ALTER TABLE ` + a.SQLTablename + ` ADD COLUMN IF NOT EXISTS name text not null default '';`,
		`ALTER TABLE ` + a.SQLTablename + ` ADD COLUMN IF NOT EXISTS url text not null default '';`,

		`ALTER TABLE ` + a.SQLTablename + `_archive  ADD COLUMN IF NOT EXISTS domain text not null  default '';`,
		`ALTER TABLE ` + a.SQLTablename + `_archive  ADD COLUMN IF NOT EXISTS name text not null  default '';`,
		`ALTER TABLE ` + a.SQLTablename + `_archive  ADD COLUMN IF NOT EXISTS url text not null  default '';`,
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
	return errors.Errorf("[table="+a.SQLTablename+", query=%s] Error: %s", q, e)
}

