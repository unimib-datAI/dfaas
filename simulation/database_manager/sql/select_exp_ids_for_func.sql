SELECT d.ExpInstantID
FROM DEPLOY     d
JOIN FUNCTION   f ON d.FunctionID = f.ID
WHERE f.Name = "{}"