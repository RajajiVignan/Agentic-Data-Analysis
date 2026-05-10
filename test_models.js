
const OpenAI = require('openai');
require('dotenv').config();

const openai = new OpenAI({
  apiKey: process.env.NVIDIA_API_KEY,
  baseURL: process.env.NVIDIA_API_BASE_URL || 'https://integrate.api.nvidia.com/v1',
});

async function listModels() {
  try {
    const models = await openai.models.list();
    console.log(JSON.stringify(models.data.map(m => m.id), null, 2));
  } catch (e) {
    console.error(e);
  }
}

listModels();
