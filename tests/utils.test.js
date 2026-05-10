
const { 
  parseCsv, 
  parseJsonRows, 
  profileRows, 
  inferType, 
  buildKpis, 
  buildTrend, 
  buildSegments 
} = require('../utils/data-processor');

describe('Data Processor Utils', () => {
  
  describe('parseCsv', () => {
    test('parses a simple CSV', () => {
      const csv = 'month,revenue\n2026-01,100\n2026-02,200';
      const result = parseCsv(csv);
      expect(result).toEqual([
        ['month', 'revenue'],
        ['2026-01', '100'],
        ['2026-02', '200']
      ]);
    });

    test('handles quoted CSV values with commas', () => {
      const csv = 'name,note\n"Doe, John","Hello, world"';
      const result = parseCsv(csv);
      expect(result).toEqual([
        ['name', 'note'],
        ['Doe, John', 'Hello, world']
      ]);
    });

    test('handles escaped quotes in CSV', () => {
      const csv = 'name,note\n"The ""Best"" App","Good"';
      const result = parseCsv(csv);
      expect(result).toEqual([
        ['name', 'note'],
        ['The "Best" App', 'Good']
      ]);
    });
  });

  describe('parseJsonRows', () => {
    test('parses an array of objects', () => {
      const json = JSON.stringify([
        { a: 1, b: 2 },
        { a: 3, b: 4 }
      ]);
      const result = parseJsonRows(json);
      expect(result[0]).toEqual(['a', 'b']);
      expect(result[1]).toEqual(['1', '2']);
    });

    test('parses JSON with data property', () => {
      const json = JSON.stringify({ data: [{ a: 1 }, { a: 2 }] });
      const result = parseJsonRows(json);
      expect(result[0]).toEqual(['a']);
      expect(result[1]).toEqual(['1']);
    });

    test('throws error on invalid JSON structure', () => {
      const json = JSON.stringify({ wrong: 'format' });
      expect(() => parseJsonRows(json)).toThrow('JSON uploads must be an array of objects');
    });
  });

  describe('inferType', () => {
    test('infers number', () => {
      expect(inferType(['1', '2.5', '3'])).toBe('number');
    });

    test('infers date', () => {
      expect(inferType(['2026-01-01', '2026-02-01', '2026-03-01'])).toBe('date');
    });

    test('infers text', () => {
      expect(inferType(['Apple', 'Banana', 'Cherry'])).toBe('text');
    });

    test('infers empty', () => {
      expect(inferType([])).toBe('empty');
    });
  });

  describe('buildKpis', () => {
    const rows = [
      { revenue: '100', segment: 'A' },
      { revenue: '200', segment: 'B' },
      { revenue: '300', segment: 'A' },
    ];
    const metricCol = { name: 'revenue' };
    const categoryCol = { name: 'segment' };

    test('calculates total and average', () => {
      const kpis = buildKpis(rows, metricCol, categoryCol);
      expect(kpis[0].value).toBe('600'); // Total
      expect(kpis[1].value).toBe('200'); // Avg
    });

    test('counts categories correctly', () => {
      const kpis = buildKpis(rows, metricCol, categoryCol);
      expect(kpis[2].value).toBe('2'); // segments A and B
    });
  });

  describe('buildTrend', () => {
    const rows = [
      { date: '2026-01-01', rev: '100' },
      { date: '2026-01-15', rev: '50' },
      { date: '2026-02-01', rev: '300' },
    ];
    const dateCol = { name: 'date' };
    const metricCol = { name: 'rev' };

    test('aggregates by month', () => {
      const trend = buildTrend(rows, dateCol, metricCol);
      expect(trend).toEqual([
        { label: '2026-01', value: 150 },
        { label: '2026-02', value: 300 },
      ]);
    });
  });

  describe('buildSegments', () => {
    const rows = [
      { seg: 'A', rev: '100' },
      { seg: 'A', rev: '100' },
      { seg: 'B', rev: '500' },
    ];
    const catCol = { name: 'seg' };
    const metricCol = { name: 'rev' };

    test('aggregates top segments by value', () => {
      const segs = buildSegments(rows, catCol, metricCol);
      expect(segs[0]).toEqual({ label: 'B', value: 500 });
      expect(segs[1]).toEqual({ label: 'A', value: 200 });
    });
  });
});
