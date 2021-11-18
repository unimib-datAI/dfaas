SELECT  m.ID, m.Type, m.Name, m.Value, m.Unit, m.Description, m.ExpInstantID,
        m.NodeID, n.Name, m.FunctionID, f.Name
FROM METRIC m
LEFT JOIN FUNCTION   f ON m.FunctionID = f.ID
LEFT JOIN NODE       n ON m.NodeID = n.ID
WHERE m.ExpInstantID IN ({})