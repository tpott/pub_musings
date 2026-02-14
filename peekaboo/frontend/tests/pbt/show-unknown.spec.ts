import { test } from '@playwright/test';
import { createPBT } from '../helpers/pbt/runner';

test('unknown subject triggers TTS fallback', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.say('what is the weather today');
  await pbt.assertNoMedia();
  await pbt.assertTTS();
});
