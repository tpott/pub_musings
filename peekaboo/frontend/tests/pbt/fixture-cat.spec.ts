import { test } from '@playwright/test';
import { createPBT } from '../helpers/pbt/runner';

// Works without Piper — uses pre-recorded audio
test('show cat from fixture', async ({ page }) => {
  const pbt = await createPBT(page);
  await pbt.sayFixture('me-show-me-a-cat.webm');
  await pbt.assertMedia('cat');
});
