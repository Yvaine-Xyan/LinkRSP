import { inject } from '@vercel/analytics';

// Initialize Vercel Analytics
// In development mode, set debug to true to see analytics events in console
inject({
  mode: 'production',
  debug: false
});
