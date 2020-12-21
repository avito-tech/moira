package neo4j

import (
	"strconv"
)

func (db *DbConnector) GetMaxDepthInGraph(id string) (int, error) {
	query := `
		MATCH
			(source:Trigger)
			WHERE NOT ()-[:IS_PARENT_OF]->(source)
		WITH source
		MATCH
			(cur:Trigger {id: $triggerID}),
			p = shortestPath((source)-[:IS_PARENT_OF*]->(cur))
		RETURN length(p)`
	parameters := map[string]interface{}{
		"triggerID": id,
	}

	response, err := db.PostQuery(query, parameters)
	if err != nil {
		return 0, err
	}

	maxDepth := 0
	for _, item := range response {
		row := item.Row
		pathLength, err := strconv.Atoi(string(row[0]))
		if err != nil {
			return 0, err
		}
		if pathLength > maxDepth {
			maxDepth = pathLength
		}
	}
	return maxDepth, nil
}

func (db *DbConnector) GetAllAncestors(id string) (allPaths [][]string, err error) {
	query := `
		MATCH
			(source:Trigger)
			WHERE NOT ()-[:IS_PARENT_OF]->(source)
		WITH source
		MATCH
			(cur:Trigger {id: $triggerID}),
			p = shortestPath((source)-[:IS_PARENT_OF*]->(cur))
		RETURN nodes(p)`
	parameters := map[string]interface{}{
		"triggerID": id,
	}

	response, err := db.PostQuery(query, parameters)
	if err != nil {
		return nil, err
	}

	allPaths = make([][]string, len(response))
	for i, item := range response {
		row := item.Row
		// skipLast is set to true because Neo4j will return the target node itself with the path
		// (here, the target node is the trigger whose ancestors we're querying for)
		// but we only want the ancestors and not the target itself
		// so let's skip it
		allPaths[i], err = db.ParsePath(row[0], true)
		if err != nil {
			return nil, err
		}
	}

	return allPaths, nil
}

func (db *DbConnector) SetTriggerParents(triggerID string, newParentIDs []string) error {
	queries := []string{
		// ensure that the child trigger exists
		`MERGE (t:Trigger {id: $childID});`,

		// ensure that the parents also all exist
		`UNWIND $parentIDs AS id MERGE (t:Trigger {id: id});`,

		// delete all existing parent relationships
		`MATCH ()-[r:IS_PARENT_OF]->(child:Trigger {id: $childID}) DELETE r;`,
		// create new parent relationships
		`MATCH (parent:Trigger), (child:Trigger {id: $childID}) WHERE parent.id IN $parentIDs MERGE (parent)-[:IS_PARENT_OF]->(child);`,
	}
	parameters := map[string]interface{}{
		"childID":   triggerID,
		"parentIDs": newParentIDs,
	}
	_, err := db.PostQueries(queries, parameters)
	if err != nil {
		return err
	}
	return nil
}

func (db *DbConnector) GetAllChildren(triggerID string) ([]string, error) {
	query := `
		MATCH
			(cur:Trigger {id: $triggerID})-[:IS_PARENT_OF]->(child:Trigger)
		RETURN child.id`
	parameters := map[string]interface{}{
		"triggerID": triggerID,
	}

	response, err := db.PostQuery(query, parameters)
	if err != nil {
		return nil, err
	}

	result := make([]string, len(response))
	for i, item := range response {
		row := item.Row
		childID := string(row[0])
		childID = childID[1 : len(childID)-1]
		result[i] = childID
	}
	return result, nil
}
