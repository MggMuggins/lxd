//go:build linux && cgo && !agent
// +build linux,cgo,!agent

package db

// The code below was generated by lxd-generate - DO NOT EDIT!

import (
	"database/sql"
	"fmt"
	"strings"

	"github.com/lxc/lxd/lxd/db/cluster"
	"github.com/lxc/lxd/lxd/db/query"
	"github.com/lxc/lxd/shared/api"
)

var _ = api.ServerEnvironment{}

var certificateObjects = cluster.RegisterStmt(`
SELECT certificates.id, certificates.fingerprint, certificates.type, certificates.name, certificates.certificate, certificates.restricted
  FROM certificates
  ORDER BY certificates.fingerprint
`)

var certificateObjectsByFingerprint = cluster.RegisterStmt(`
SELECT certificates.id, certificates.fingerprint, certificates.type, certificates.name, certificates.certificate, certificates.restricted
  FROM certificates
  WHERE certificates.fingerprint = ? ORDER BY certificates.fingerprint
`)

var certificateID = cluster.RegisterStmt(`
SELECT certificates.id FROM certificates
  WHERE certificates.fingerprint = ?
`)

var certificateCreate = cluster.RegisterStmt(`
INSERT INTO certificates (fingerprint, type, name, certificate, restricted)
  VALUES (?, ?, ?, ?, ?)
`)

var certificateDeleteByFingerprint = cluster.RegisterStmt(`
DELETE FROM certificates WHERE fingerprint = ?
`)

var certificateDeleteByNameAndType = cluster.RegisterStmt(`
DELETE FROM certificates WHERE name = ? AND type = ?
`)

var certificateUpdate = cluster.RegisterStmt(`
UPDATE certificates
  SET fingerprint = ?, type = ?, name = ?, certificate = ?, restricted = ?
 WHERE id = ?
`)

// GetCertificates returns all available certificates.
// generator: certificate GetMany
func (c *ClusterTx) GetCertificates(filter CertificateFilter) ([]Certificate, error) {
	var err error

	// Result slice.
	objects := make([]Certificate, 0)

	// Pick the prepared statement and arguments to use based on active criteria.
	var stmt *sql.Stmt
	var args []interface{}

	if filter.Fingerprint != nil && filter.Name == nil && filter.Type == nil {
		stmt = c.stmt(certificateObjectsByFingerprint)
		args = []interface{}{
			filter.Fingerprint,
		}
	} else if filter.Fingerprint == nil && filter.Name == nil && filter.Type == nil {
		stmt = c.stmt(certificateObjects)
		args = []interface{}{}
	} else {
		return nil, fmt.Errorf("No statement exists for the given Filter")
	}

	// Dest function for scanning a row.
	dest := func(i int) []interface{} {
		objects = append(objects, Certificate{})
		return []interface{}{
			&objects[i].ID,
			&objects[i].Fingerprint,
			&objects[i].Type,
			&objects[i].Name,
			&objects[i].Certificate,
			&objects[i].Restricted,
		}
	}

	// Select.
	err = query.SelectObjects(stmt, dest, args...)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch from \"certificates\" table: %w", err)
	}

	certificateProjects, err := c.GetCertificateProjects()
	if err != nil {
		return nil, err
	}

	for i := range objects {
		objects[i].Projects = make([]string, 0)
		if refIDs, ok := certificateProjects[objects[i].ID]; ok {
			for _, refID := range refIDs {
				projectURIs, err := c.GetProjectURIs(ProjectFilter{ID: &refID})
				if err != nil {
					return nil, err
				}

				for i, uri := range projectURIs {
					if strings.HasPrefix(uri, "/1.0/") {
						uri = strings.Split(uri, "/1.0/projects/")[1]
						uri = strings.Split(uri, "?")[0]
						projectURIs[i] = uri
					}
				}
				objects[i].Projects = append(objects[i].Projects, projectURIs...)
			}
		}
	}

	return objects, nil
}

// GetCertificate returns the certificate with the given key.
// generator: certificate GetOne
func (c *ClusterTx) GetCertificate(fingerprint string) (*Certificate, error) {
	filter := CertificateFilter{}
	filter.Fingerprint = &fingerprint

	objects, err := c.GetCertificates(filter)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch from \"certificates\" table: %w", err)
	}

	switch len(objects) {
	case 0:
		return nil, ErrNoSuchObject
	case 1:
		return &objects[0], nil
	default:
		return nil, fmt.Errorf("More than one \"certificates\" entry matches")
	}
}

// GetCertificateID return the ID of the certificate with the given key.
// generator: certificate ID
func (c *ClusterTx) GetCertificateID(fingerprint string) (int64, error) {
	stmt := c.stmt(certificateID)
	rows, err := stmt.Query(fingerprint)
	if err != nil {
		return -1, fmt.Errorf("Failed to get \"certificates\" ID: %w", err)
	}

	defer rows.Close()

	// Ensure we read one and only one row.
	if !rows.Next() {
		return -1, ErrNoSuchObject
	}
	var id int64
	err = rows.Scan(&id)
	if err != nil {
		return -1, fmt.Errorf("Failed to scan ID: %w", err)
	}

	if rows.Next() {
		return -1, fmt.Errorf("More than one row returned")
	}
	err = rows.Err()
	if err != nil {
		return -1, fmt.Errorf("Result set failure: %w", err)
	}

	return id, nil
}

// CertificateExists checks if a certificate with the given key exists.
// generator: certificate Exists
func (c *ClusterTx) CertificateExists(fingerprint string) (bool, error) {
	_, err := c.GetCertificateID(fingerprint)
	if err != nil {
		if err == ErrNoSuchObject {
			return false, nil
		}
		return false, err
	}

	return true, nil
}

// CreateCertificate adds a new certificate to the database.
// generator: certificate Create
func (c *ClusterTx) CreateCertificate(object Certificate) (int64, error) {
	// Check if a certificate with the same key exists.
	exists, err := c.CertificateExists(object.Fingerprint)
	if err != nil {
		return -1, fmt.Errorf("Failed to check for duplicates: %w", err)
	}

	if exists {
		return -1, fmt.Errorf("This \"certificates\" entry already exists")
	}

	args := make([]interface{}, 5)

	// Populate the statement arguments.
	args[0] = object.Fingerprint
	args[1] = object.Type
	args[2] = object.Name
	args[3] = object.Certificate
	args[4] = object.Restricted

	// Prepared statement to use.
	stmt := c.stmt(certificateCreate)

	// Execute the statement.
	result, err := stmt.Exec(args...)
	if err != nil {
		return -1, fmt.Errorf("Failed to create \"certificates\" entry: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return -1, fmt.Errorf("Failed to fetch \"certificates\" entry ID: %w", err)
	}

	// Update association table.
	object.ID = int(id)
	err = c.UpdateCertificateProjects(object)
	if err != nil {
		return -1, fmt.Errorf("Could not update association table: %w", err)
	}

	return id, nil
}

// DeleteCertificate deletes the certificate matching the given key parameters.
// generator: certificate DeleteOne-by-Fingerprint
func (c *ClusterTx) DeleteCertificate(fingerprint string) error {
	stmt := c.stmt(certificateDeleteByFingerprint)
	result, err := stmt.Exec(fingerprint)
	if err != nil {
		return fmt.Errorf("Delete \"certificates\": %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Fetch affected rows: %w", err)
	}

	if n != 1 {
		return fmt.Errorf("Query deleted %d rows instead of 1", n)
	}

	return nil
}

// DeleteCertificates deletes the certificate matching the given key parameters.
// generator: certificate DeleteMany-by-Name-and-Type
func (c *ClusterTx) DeleteCertificates(name string, certificateType CertificateType) error {
	stmt := c.stmt(certificateDeleteByNameAndType)
	result, err := stmt.Exec(name, certificateType)
	if err != nil {
		return fmt.Errorf("Delete \"certificates\": %w", err)
	}

	_, err = result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Fetch affected rows: %w", err)
	}

	return nil
}

// UpdateCertificate updates the certificate matching the given key parameters.
// generator: certificate Update
func (c *ClusterTx) UpdateCertificate(fingerprint string, object Certificate) error {
	id, err := c.GetCertificateID(fingerprint)
	if err != nil {
		return err
	}

	stmt := c.stmt(certificateUpdate)
	result, err := stmt.Exec(object.Fingerprint, object.Type, object.Name, object.Certificate, object.Restricted, id)
	if err != nil {
		return fmt.Errorf("Update \"certificates\" entry failed: %w", err)
	}

	n, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("Fetch affected rows: %w", err)
	}

	if n != 1 {
		return fmt.Errorf("Query updated %d rows instead of 1", n)
	}

	// Update association table.
	object.ID = int(id)
	err = c.UpdateCertificateProjects(object)
	if err != nil {
		return fmt.Errorf("Could not update association table: %w", err)
	}

	return nil
}
