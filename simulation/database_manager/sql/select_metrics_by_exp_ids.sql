SELECT m.Name, m.type, f.Name, n.Name, AVG(m.Value), d.MaxRate, d.NumReplicas, d.Margin, d.State
FROM METRIC m
LEFT JOIN FUNCTION   f ON m.FunctionID = f.ID
LEFT JOIN NODE       n ON m.NodeID = n.ID
LEFT JOIN DEPLOY     d ON m.ExpInstantID = d.ExpInstantID and
                          f.ID = d.FunctionID
WHERE m.ExpInstantID IN ({})
GROUP BY m.Name, m.type, f.Name, n.Name, d.MaxRate, d.NumReplicas, d.Margin, d.State