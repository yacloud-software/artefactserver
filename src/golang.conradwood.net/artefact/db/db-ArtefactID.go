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

 CREATE TABLE artefactid (id integer primary key default nextval('artefactid_seq'),domain varchar(2000) not null,name varchar(2000) not null);

Archive Table: (structs can be moved from main to archive using Archive() function)

 CREATE TABLE artefactid_archive (id integer unique not null,domain varchar(2000) not null,name varchar(2000) not null);
*/

import (
	gosql "database/sql"
	"fmt"
	savepb "golang.conradwood.net/apis/artefact"
	"golang.conradwood.net/go-easyops/sql"
	"golang.org/x/net/context"
)

type DBArtefactID struct {
	DB *sql.DB
}

func NewDBArtefactID(db *sql.DB) *DBArtefactID {
	foo := DBArtefactID{DB: db}
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
	_, e := a.DB.ExecContext(ctx, "insert_DBArtefactID", "insert into artefactid_archive (id,domain, name) values ($1,$2, $3) ", p.ID, p.Domain, p.Name)
	if e != nil {
		return e
	}

	// now delete it.
	a.DeleteByID(ctx, id)
	return nil
}

// Save (and use database default ID generation)
func (a *DBArtefactID) Save(ctx context.Context, p *savepb.ArtefactID) (uint64, error) {
	rows, e := a.DB.QueryContext(ctx, "DBArtefactID_Save", "insert into artefactid (domain, name) values ($1, $2) returning id", p.Domain, p.Name)
	if e != nil {
		return 0, e
	}
	defer rows.Close()
	if !rows.Next() {
		return 0, fmt.Errorf("No rows after insert")
	}
	var id uint64
	e = rows.Scan(&id)
	if e != nil {
		return 0, fmt.Errorf("failed to scan id after insert: %s", e)
	}
	p.ID = id
	return id, nil
}

// Save using the ID specified
func (a *DBArtefactID) SaveWithID(ctx context.Context, p *savepb.ArtefactID) error {
	_, e := a.DB.ExecContext(ctx, "insert_DBArtefactID", "insert into artefactid (id,domain, name) values ($1,$2, $3) ", p.ID, p.Domain, p.Name)
	return e
}

func (a *DBArtefactID) Update(ctx context.Context, p *savepb.ArtefactID) error {
	_, e := a.DB.ExecContext(ctx, "DBArtefactID_Update", "update artefactid set domain=$1, name=$2 where id = $3", p.Domain, p.Name, p.ID)

	return e
}

// delete by id field
func (a *DBArtefactID) DeleteByID(ctx context.Context, p uint64) error {
	_, e := a.DB.ExecContext(ctx, "deleteDBArtefactID_ByID", "delete from artefactid where id = $1", p)
	return e
}

// get it by primary id
func (a *DBArtefactID) ByID(ctx context.Context, p uint64) (*savepb.ArtefactID, error) {
	rows, e := a.DB.QueryContext(ctx, "DBArtefactID_ByID", "select id,domain, name from artefactid where id = $1", p)
	if e != nil {
		return nil, fmt.Errorf("ByID: error querying (%s)", e)
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, fmt.Errorf("ByID: error scanning (%s)", e)
	}
	if len(l) == 0 {
		return nil, fmt.Errorf("No ArtefactID with id %d", p)
	}
	if len(l) != 1 {
		return nil, fmt.Errorf("Multiple (%d) ArtefactID with id %d", len(l), p)
	}
	return l[0], nil
}

// get all rows
func (a *DBArtefactID) All(ctx context.Context) ([]*savepb.ArtefactID, error) {
	rows, e := a.DB.QueryContext(ctx, "DBArtefactID_all", "select id,domain, name from artefactid order by id")
	if e != nil {
		return nil, fmt.Errorf("All: error querying (%s)", e)
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
	rows, e := a.DB.QueryContext(ctx, "DBArtefactID_ByDomain", "select id,domain, name from artefactid where domain = $1", p)
	if e != nil {
		return nil, fmt.Errorf("ByDomain: error querying (%s)", e)
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, fmt.Errorf("ByDomain: error scanning (%s)", e)
	}
	return l, nil
}

// get all "DBArtefactID" rows with matching Name
func (a *DBArtefactID) ByName(ctx context.Context, p string) ([]*savepb.ArtefactID, error) {
	rows, e := a.DB.QueryContext(ctx, "DBArtefactID_ByName", "select id,domain, name from artefactid where name = $1", p)
	if e != nil {
		return nil, fmt.Errorf("ByName: error querying (%s)", e)
	}
	defer rows.Close()
	l, e := a.FromRows(ctx, rows)
	if e != nil {
		return nil, fmt.Errorf("ByName: error scanning (%s)", e)
	}
	return l, nil
}

/**********************************************************************
* Helper to convert from an SQL Row to struct
**********************************************************************/
func (a *DBArtefactID) Tablename() string {
	return "artefactid"
}

func (a *DBArtefactID) SelectCols() string {
	return "id,domain, name"
}

func (a *DBArtefactID) FromRows(ctx context.Context, rows *gosql.Rows) ([]*savepb.ArtefactID, error) {
	var res []*savepb.ArtefactID
	for rows.Next() {
		foo := savepb.ArtefactID{}
		err := rows.Scan(&foo.ID, &foo.Domain, &foo.Name)
		if err != nil {
			return nil, err
		}
		res = append(res, &foo)
	}
	return res, nil
}
