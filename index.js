const express = require('express');
const cors = require('cors');
const initSqlJs = require('sql.js');
const path = require('path');

const app = express();
const PORT = process.env.PORT || 3000;

// CORS block hatane ke liye middleware
app.use(cors());
app.use(express.json());
app.use(express.static(__dirname));

let db;

initSqlJs()
  .then((SQL) => {
    db = new SQL.Database();
    console.log('Database initialized successfully!');
  })
  .catch((err) => console.error('Database Init Error:', err));

app.get('/api/tables', (req, res) => {
  if (!db) return res.status(500).json({ error: 'Database not ready yet' });
  try {
    const resTables = db.exec(
      "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
    );
    const tables =
      resTables.length > 0
        ? resTables[0].values.map((row) => ({ name: row[0] }))
        : [];
    res.json(tables);
  } catch (err) {
    res.status(500).json({ error: err.message });
  }
});

app.post('/api/query', (req, res) => {
  if (!db) return res.status(500).json({ error: 'Database not ready yet' });

  let { query } = req.body;
  if (!query) return res.status(400).json({ error: 'No query provided' });

  // Semicolon se queries split karein
  const queries = query.split(';').filter((q) => q.trim().length > 0);
  let finalResult = null;
  let errorOccurred = null;

  for (let q of queries) {
    let sql = q.trim();

    // 1. TRUNCATE FIX: TRUNCATE TABLE -> DELETE FROM
    if (/^TRUNCATE\s+TABLE/i.test(sql)) {
      sql = sql.replace(/TRUNCATE\s+TABLE/i, 'DELETE FROM');
    }

    // 2. CREATE TABLE FIX: Auto-append IF NOT EXISTS (Table Existing Error Fix)
    if (/^CREATE\s+TABLE\s+(?!IF\s+NOT\s+EXISTS)/i.test(sql)) {
      sql = sql.replace(/^CREATE\s+TABLE\s+/i, 'CREATE TABLE IF NOT EXISTS ');
    }

    // 3. INSERT FIX: INSERT INTO -> INSERT OR REPLACE (Unique Constraint Error Fix)
    if (/^INSERT\s+INTO/i.test(sql) && !/INSERT\s+OR\s+REPLACE/i.test(sql)) {
      sql = sql.replace(/^INSERT\s+INTO/i, 'INSERT OR REPLACE INTO');
    }

    try {
      const firstWord = sql.split(' ')[0].toUpperCase();

      if (firstWord === 'SELECT') {
        const queryResult = db.exec(sql);
        if (queryResult.length > 0) {
          finalResult = {
            columns: queryResult[0].columns,
            rows: queryResult[0].values,
          };
        } else {
          finalResult = { columns: [], rows: [] };
        }
      } else {
        db.run(sql);
        if (!finalResult) {
          finalResult = { message: `✅ Queries Executed Successfully!` };
        }
      }
    } catch (err) {
      errorOccurred = err.message;
      break;
    }
  }

  if (errorOccurred) {
    res.json({ error: errorOccurred });
  } else {
    res.json(finalResult || { message: '✅ Queries Executed Successfully!' });
  }
});

app.get('/', (req, res) => {
  res.sendFile(path.join(__dirname, 'index.html'));
});

app.listen(PORT, () => {
  console.log(`Server running on http://localhost:${PORT}`);
});
