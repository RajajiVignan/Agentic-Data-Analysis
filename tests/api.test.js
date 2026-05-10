
const request = require('supertest');
const server = require('../server');

describe('InsightPilot API', () => {
  // Increase global timeout for AI calls
  jest.setTimeout(30000);

  test('GET /api/health returns 200', async () => {
    const res = await request(server).get('/api/health');
    expect(res.statusCode).toEqual(200);
    expect(res.body).toHaveProperty('ok', true);
  });

  test('GET /api/datasets returns 200', async () => {
    const res = await request(server).get('/api/datasets');
    expect(res.statusCode).toEqual(200);
    expect(Array.isArray(res.body.datasets)).toBe(true);
  });

  test('POST /api/connect-source creates a dataset', async () => {
    const res = await request(server)
      .post('/api/connect-source')
      .send({ source: 'Test Warehouse' });
    
    expect(res.statusCode).toEqual(201);
    expect(res.body).toHaveProperty('datasetId');
    expect(res.body).toHaveProperty('filename');
  });

  test('POST /api/analyze returns analysis for valid dataset', async () => {
    const setupRes = await request(server)
      .post('/api/connect-source')
      .send({ source: 'Analysis Test' });
    
    const datasetId = setupRes.body.datasetId;
    
    const res = await request(server)
      .post('/api/analyze')
      .send({ datasetId, prompt: 'What is the total revenue?' });
    
    expect(res.statusCode).toEqual(200);
    expect(res.body).toHaveProperty('notebook');
    expect(res.body).toHaveProperty('dashboard');
  });

  test('POST /api/analyze returns 404 for missing dataset', async () => {
    const res = await request(server)
      .post('/api/analyze')
      .send({ datasetId: 'non-existent-id', prompt: 'Hi' });
    
    expect(res.statusCode).toEqual(404);
    expect(res.body).toHaveProperty('error');
  });

  test('POST /api/upload handles invalid multipart', async () => {
    const res = await request(server)
      .post('/api/upload')
      .set('Content-Type', 'multipart/form-data; boundary=wrong')
      .send('not-a-real-upload');
    
    expect(res.statusCode).toEqual(400);
  });
});
