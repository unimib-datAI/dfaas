SELECT e.ID
FROM NODE n
JOIN EXPERIMENT_INSTANT e ON n.ID = e.NodeID
JOIN DEPLOY             d ON e.ID = d.ExpInstantID
JOIN FUNCTION           f ON d.FunctionID = f.ID
WHERE {}
GROUP BY e.ID
HAVING COUNT(f.Name) = {}