import { createClient } from '@supabase/supabase-js';
import dotenv from 'dotenv';

dotenv.config();

const supabaseUrl = process.env.SUPABASE_URL;
const supabaseKey = process.env.SUPABASE_KEY;

if (!supabaseUrl || !supabaseKey) {
  throw new Error('SUPABASE_URL and SUPABASE_KEY must be defined in environment variables.');
}

export const supabase = createClient(supabaseUrl, supabaseKey);

export const healthCheck = async () => {
  try {
    const { data, error } = await supabase
      .from('tables')
      .select('*', { count: 'exact', head: true });
    
    if (error) throw error;
    return { status: 'connected' };
  } catch (e) {
    return { status: 'error', message: e.message };
  }
};