import { test } from '@playwright/test';
import { createPBT } from '../helpers/pbt/runner';

test('show me a cat displays cat media', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.say('show me a cat');
  await pbt.assertMedia('cat');
  await pbt.assertTranscript('cat');
  await pbt.assertNoTTS();
});
