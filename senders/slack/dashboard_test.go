package slack

import (
	"testing"

	"go.avito.ru/DO/moira"
)

func TestRenderDashboardForSlack(t *testing.T) {
	type TestCase struct {
		Dashboard      moira.SlackDashboard
		MaxLines       int
		ExpectedRender string
	}

	testDashboard := moira.SlackDashboard(
		map[string]bool{
			"metric-9": true,
			"metric-8": false,
			"metric-7": true,
			"metric-6": false,
			"metric-5": true,
			"metric-4": false,
			"metric-3": true,
			"metric-2": false,
			"metric-1": true,
		},
	)
	testCases := []TestCase{
		{
			Dashboard: testDashboard,
			MaxLines:  0,
			ExpectedRender: ":x: metric-2\n" +
				":x: metric-4\n" +
				":x: metric-6\n" +
				":x: metric-8\n" +
				":jr-approve: metric-1\n" +
				":jr-approve: metric-3\n" +
				":jr-approve: metric-5\n" +
				":jr-approve: metric-7\n" +
				":jr-approve: metric-9\n",
		},

		{
			Dashboard: testDashboard,
			MaxLines:  5,
			ExpectedRender: ":x: metric-2\n" +
				":x: metric-4\n" +
				":x: metric-6\n" +
				":x: metric-8\n" +
				":jr-approve: metric-1\n" +
				"(and 4 more: 0 :x:, 4 :jr-approve:)\n",
		},

		{
			Dashboard: testDashboard,
			MaxLines:  3,
			ExpectedRender: ":x: metric-2\n" +
				":x: metric-4\n" +
				":x: metric-6\n" +
				"(and 6 more: 1 :x:, 5 :jr-approve:)\n",
		},
	}

	for i, testCase := range testCases {
		render := RenderDashboardForSlack(testCase.Dashboard, testCase.MaxLines)
		if render != testCase.ExpectedRender {
			t.Errorf("Test case %d, render = %#v, expected %#v", i, render, testCase.ExpectedRender)
		}
	}
}
